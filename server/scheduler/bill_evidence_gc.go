// 账单凭证孤儿文件清理。
//
// 删除语义（与 handlers/bills.go 约定一致）：
//   - 管理员移除凭证 / 更新账单整单替换：只删 bill_evidences 行，物理文件保留；
//   - 账单软删除：凭证行保留（账单可恢复）；账单被彻底删除（如 90 天清理）时行随 CASCADE 消失；
//   - 上传成功但未随账单提交：只有文件、没有行。
//
// 因此存储里会存在「无行引用」的孤儿文件，本任务统一回收：
// 遍历 bills 目录下的对象，凡「未被任何 bill_evidences.file_key 引用」且
// 「写入时间早于宽限期（默认 7 天）」的一律删除。宽限期给「上传后隔几天才提交表单」
// 和并发中的新上传留缓冲。
package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/models"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"gorm.io/gorm"
)

// BillEvidenceGCGrace 孤儿文件宽限期。
const BillEvidenceGCGrace = 7 * 24 * time.Hour

// CleanupBillEvidenceFiles 清理孤儿凭证文件，返回执行摘要。
func CleanupBillEvidenceFiles(db *gorm.DB, cfg *config.Config) string {
	referenced := map[string]bool{}
	rows, err := db.Model(&models.BillEvidence{}).Select("file_key").Rows()
	if err != nil {
		logger.Log.Error().Err(err).Msg("凭证GC: 查询引用失败")
		return "凭证GC: 失败（查询引用）"
	}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err == nil && key != "" {
			referenced[key] = true
		}
	}
	rows.Close()

	cutoff := time.Now().Add(-BillEvidenceGCGrace)
	deleted, checked := 0, 0
	if cfg.Storage.Qiniu.AccessKey != "" && cfg.Storage.Qiniu.Bucket != "" {
		deleted, checked, err = gcQiniu(cfg, referenced, cutoff)
	} else {
		deleted, checked, err = gcLocal(cfg.Server.UploadDir, referenced, cutoff)
	}
	if err != nil {
		logger.Log.Error().Err(err).Msg("凭证GC: 遍历存储失败")
		return "凭证GC: 失败（遍历存储）"
	}
	summary := fmt.Sprintf("凭证GC: 扫描%d个文件，清理%d个孤儿", checked, deleted)
	logger.Log.Info().Int("checked", checked).Int("deleted", deleted).Msg(summary)
	return summary
}

// gcLocal 本地存储模式：遍历 UploadDir 下 buildings/*/bills/ 目录。
func gcLocal(uploadDir string, referenced map[string]bool, cutoff time.Time) (deleted, checked int, err error) {
	root := filepath.Join(uploadDir, "buildings")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	for _, building := range entries {
		if !building.IsDir() {
			continue
		}
		billDir := filepath.Join(root, building.Name(), "bills")
		files, err := os.ReadDir(billDir)
		if err != nil {
			continue // 无 bills 目录属正常
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			checked++
			key := filepath.ToSlash(filepath.Join("buildings", building.Name(), "bills", f.Name()))
			if referenced[key] {
				continue
			}
			info, err := f.Info()
			if err != nil || info.ModTime().After(cutoff) {
				continue // 宽限期内
			}
			if rmErr := os.Remove(filepath.Join(billDir, f.Name())); rmErr != nil {
				logger.Log.Warn().Err(rmErr).Str("key", key).Msg("凭证GC: 删除本地文件失败")
				continue
			}
			deleted++
		}
	}
	return deleted, checked, nil
}

// qiniuRegion 区域名转 SDK Region（与 handlers.getZone 口径一致；该函数未导出，此处独立实现）。
func qiniuRegion(name string) *storage.Region {
	switch name {
	case "z0":
		return &storage.ZoneHuadong
	case "z1":
		return &storage.ZoneHuabei
	case "z2":
		return &storage.ZoneHuanan
	case "na0":
		return &storage.ZoneBeimei
	default:
		return &storage.ZoneHuadong
	}
}

// gcQiniu 七牛模式：列举 bucket 下 buildings/ 前缀，按路径段过滤 bills 目录。
func gcQiniu(cfg *config.Config, referenced map[string]bool, cutoff time.Time) (deleted, checked int, err error) {
	mac := qbox.NewMac(cfg.Storage.Qiniu.AccessKey, cfg.Storage.Qiniu.SecretKey)
	qcfg := storage.Config{Region: qiniuRegion(cfg.Storage.Qiniu.Zone), UseHTTPS: cfg.Storage.Qiniu.UseHTTPS}
	bucketMgr := storage.NewBucketManager(mac, &qcfg)

	prefix := "buildings/"
	marker := ""
	for {
		entries, _, nextMarker, hasNext, err := bucketMgr.ListFiles(cfg.Storage.Qiniu.Bucket, prefix, "", marker, 1000)
		if err != nil {
			return deleted, checked, err
		}
		for _, e := range entries {
			// key 形如 buildings/{bid}_{slug}/bills/{uuid}{ext}，第三段必须是 bills
			parts := strings.Split(e.Key, "/")
			if len(parts) < 4 || parts[2] != "bills" {
				continue
			}
			checked++
			if referenced[e.Key] || e.PutTime == 0 {
				continue
			}
			// 七牛 putTime 为 100 纳秒单位
			putTime := time.Unix(0, e.PutTime*100)
			if putTime.After(cutoff) {
				continue
			}
			if delErr := bucketMgr.Delete(cfg.Storage.Qiniu.Bucket, e.Key); delErr != nil {
				logger.Log.Warn().Err(delErr).Str("key", e.Key).Msg("凭证GC: 删除七牛文件失败")
				continue
			}
			deleted++
		}
		if !hasNext || nextMarker == "" {
			break
		}
		marker = nextMarker
	}
	return deleted, checked, nil
}
