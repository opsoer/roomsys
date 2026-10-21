package scheduler

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/qiniu"

	"gorm.io/gorm"
)

// backupPrefix 对象存储备份文件的 key 前缀（与媒体的 buildings/ 目录隔离）。
const backupPrefix = "backups/db/"

// BackupDatabase 全量备份数据库并上传到对象存储。
// sqlite：VACUUM INTO 产出自洽单文件快照（WAL 模式下不能裸拷 rental.db）；
// mysql：调用 mysqldump 外部命令（部署机未安装时给出明确提示）。
// key 带随机后缀防枚举（备份含房东电话、银行卡等敏感数据）。
func BackupDatabase(db *gorm.DB, cfg *config.Config) string {
	if !qiniu.Enabled(cfg) {
		return "备份失败：未配置七牛对象存储（storage.qiniu.access_key/secret_key/bucket）"
	}

	data, err := dumpDatabase(db, cfg)
	if err != nil {
		logger.Log.Error().Err(err).Msg("数据库备份导出失败")
		return "备份失败：" + err.Error()
	}

	var gzBuf bytes.Buffer
	zw := gzip.NewWriter(&gzBuf)
	if _, err := zw.Write(data); err != nil {
		return "备份失败：gzip 压缩出错 " + err.Error()
	}
	if err := zw.Close(); err != nil {
		return "备份失败：gzip 关闭出错 " + err.Error()
	}

	key := backupPrefix + backupObjectName()
	if err := qiniu.UploadBytes(cfg, key, gzBuf.Bytes()); err != nil {
		logger.Log.Error().Err(err).Str("key", key).Msg("备份上传对象存储失败")
		return "备份失败：上传对象存储出错 " + err.Error()
	}

	pruned := pruneBackups(cfg)
	size := gzBuf.Len()
	logger.Log.Info().Str("key", key).Int("bytes", size).Msg("数据库备份完成")
	return fmt.Sprintf("备份完成：%s（%.1f KB，gzip）%s", key, float64(size)/1024, pruned)
}

// dumpDatabase 按驱动导出全库内容到内存。
func dumpDatabase(db *gorm.DB, cfg *config.Config) ([]byte, error) {
	switch cfg.Database.Driver {
	case config.DriverSQLite:
		return dumpSQLite(db)
	case config.DriverMySQL:
		return dumpMySQL(cfg)
	default:
		return nil, fmt.Errorf("不支持的数据库驱动: %s", cfg.Database.Driver)
	}
}

// dumpSQLite 用 VACUUM INTO 产出自洽快照文件后读入内存（库仅几 MB，可整体驻留）。
func dumpSQLite(db *gorm.DB) ([]byte, error) {
	dir, err := os.MkdirTemp("", "rental-backup-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	snap := filepath.Join(dir, "snapshot.db")
	// VACUUM INTO 目标文件必须不存在；路径中的单引号按 SQL 字面量规则转义。
	if err := db.Exec(fmt.Sprintf("VACUUM INTO '%s'", strings.ReplaceAll(snap, "'", "''"))).Error; err != nil {
		return nil, fmt.Errorf("VACUUM INTO 失败: %w", err)
	}
	data, err := os.ReadFile(snap)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// dumpMySQL 调用 mysqldump 导出。找不到二进制时给出可操作的提示。
func dumpMySQL(cfg *config.Config) ([]byte, error) {
	m := cfg.Database.MySQL
	args := []string{"-h", m.Host, "-P", m.Port, "-u", m.User, "--single-transaction", "--quick", m.Name}
	cmd := exec.Command("mysqldump", args...)
	if m.Password != "" {
		// 密码经环境变量传递，避免出现在进程命令行（ps/任务管理器可见）
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+m.Password)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.Error); ok && e.Err == exec.ErrNotFound {
			return nil, fmt.Errorf("未找到 mysqldump 命令，请在服务器安装 mysql 客户端工具或手动备份")
		}
		return nil, fmt.Errorf("mysqldump 执行失败: %v（%s）", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// backupObjectName 生成备份对象名：rental-YYYYMMDD-HHMMSS-{16位随机hex}.gz，
// 随机后缀使 key 不可被时间戳枚举（备份含敏感数据）。时间戳取真实时间而非模拟时间：
// 保留策略按 key 字典序删旧，模拟时钟回拨/跳变会打乱顺序、误删最新备份。
func backupObjectName() string {
	now := time.Now()
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("rental-%s-%s.gz", now.Format("20060102-150405"), hex.EncodeToString(b[:]))
}

// pruneBackups 清理对象存储中超出保留份数的旧备份，返回清理说明（无清理时为空）。
func pruneBackups(cfg *config.Config) string {
	keep := cfg.Scheduler.DBBackup.Keep
	if keep <= 0 {
		return ""
	}
	keys, err := qiniu.ListKeys(cfg, backupPrefix)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("列出备份文件失败，跳过清理")
		return "（旧备份清理失败）"
	}
	var ours []string
	for _, k := range keys {
		if strings.HasPrefix(k, backupPrefix+"rental-") && strings.HasSuffix(k, ".gz") {
			ours = append(ours, k)
		}
	}
	// key 中的时间戳可按字典序定序（rental-YYYYMMDD-HHMMSS-…）
	sort.Strings(ours)
	if len(ours) <= keep {
		return ""
	}
	old := ours[:len(ours)-keep]
	if err := qiniu.Delete(cfg, old...); err != nil {
		logger.Log.Warn().Err(err).Msg("删除过期备份失败")
		return fmt.Sprintf("（已上传，但清理 %d 份旧备份失败）", len(old))
	}
	logger.Log.Info().Int("count", len(old)).Msg("已清理过期备份")
	return fmt.Sprintf("，已清理 %d 份过期备份", len(old))
}
