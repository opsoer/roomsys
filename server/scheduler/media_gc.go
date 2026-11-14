// 房间媒体 / 公寓封面孤儿文件清理。
//
// 背景：删除房间或公寓时，只会回收「本房间/本公寓独占」的媒体文件（按引用计数判断），
// 但下面这些路径会在对象存储（及本地磁盘）里留下再也没人引用的孤儿文件：
//   - 上传成功但建记录失败、事务回滚、进程被杀；
//   - 封面被替换或清空时旧文件未回收（历史实现只改字段不删文件）；
//   - 删公寓后残留的空目录树。
//
// 本任务遍历存储中 buildings/ 下的媒体文件（rooms/ 与 cover/ 两类子树），
// 凡「未被任何 room_media.file_path / room_media.thumbnail_path / buildings.cover_image 引用」
// 且「写入时间早于宽限期（默认 7 天）」的一律删除。宽限期给「上传后隔几天才落库」
// 与并发中的新上传留缓冲。
//
// 注意：账单凭证（buildings/*/bills/）不在本任务范围内，由 bill_evidence_gc 单独回收。
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
	"rental-server/qiniu"

	"gorm.io/gorm"
)

// MediaGCGrace 孤儿文件宽限期。
const MediaGCGrace = 7 * 24 * time.Hour

// mediaKeyPrefixes 媒体文件所在的路径片段（相对 buildings/ 前缀）。
var mediaKeyPrefixes = []string{"/rooms/", "/cover/"}

// isMediaKey 判断 key 是否属于房间媒体/封面（排除账单凭证目录 bills/）。
func isMediaKey(key string) bool {
	for _, seg := range mediaKeyPrefixes {
		if strings.Contains(key, seg) {
			return true
		}
	}
	return false
}

// CleanupMediaFiles 清理孤儿媒体文件，返回执行摘要。
func CleanupMediaFiles(db *gorm.DB, cfg *config.Config) string {
	referenced, err := collectMediaReferences(db)
	if err != nil {
		logger.Log.Error().Err(err).Msg("媒体GC: 查询引用失败")
		return "媒体GC: 失败（查询引用）"
	}
	cutoff := time.Now().Add(-MediaGCGrace)
	var deleted, checked int
	if qiniu.Enabled(cfg) {
		deleted, checked, err = gcQiniuMedia(cfg, referenced, cutoff)
	} else {
		deleted, checked, err = gcLocalMedia(cfg.Server.UploadDir, referenced, cutoff)
	}
	if err != nil {
		logger.Log.Error().Err(err).Msg("媒体GC: 遍历存储失败")
		return "媒体GC: 失败（遍历存储）"
	}
	summary := fmt.Sprintf("媒体GC: 扫描%d个文件，清理%d个孤儿", checked, deleted)
	logger.Log.Info().Int("checked", checked).Int("deleted", deleted).Msg(summary)
	return summary
}

// collectMediaReferences 收集所有仍被引用的媒体 key：
// room_media 的原图与缩略图、公寓封面。
func collectMediaReferences(db *gorm.DB) (map[string]bool, error) {
	referenced := map[string]bool{}
	add := func(rows *[]string, err error) error {
		if err != nil {
			return err
		}
		for _, k := range *rows {
			if k != "" {
				referenced[k] = true
			}
		}
		return nil
	}
	var files, thumbs, covers []string
	if err := db.Model(&models.RoomMedia{}).Where("file_path <> ''").Pluck("file_path", &files).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.RoomMedia{}).Where("thumbnail_path <> ''").Pluck("thumbnail_path", &thumbs).Error; err != nil {
		return nil, err
	}
	// 公寓封面：软删的公寓行也算引用（行还在，文件就还不能删）
	if err := db.Unscoped().Model(&models.Building{}).Where("cover_image <> ''").Pluck("cover_image", &covers).Error; err != nil {
		return nil, err
	}
	if err := add(&files, nil); err != nil {
		return nil, err
	}
	if err := add(&thumbs, nil); err != nil {
		return nil, err
	}
	if err := add(&covers, nil); err != nil {
		return nil, err
	}
	return referenced, nil
}

// gcLocalMedia 本地存储模式：遍历 UploadDir/buildings/ 下的 rooms/ 与 cover/ 文件。
func gcLocalMedia(uploadDir string, referenced map[string]bool, cutoff time.Time) (deleted, checked int, err error) {
	root := filepath.Join(uploadDir, "buildings")
	if _, statErr := os.Stat(root); statErr != nil {
		if os.IsNotExist(statErr) {
			return 0, 0, nil
		}
		return 0, 0, statErr
	}
	buildings, err := os.ReadDir(root)
	if err != nil {
		return 0, 0, err
	}
	for _, b := range buildings {
		if !b.IsDir() {
			continue
		}
		bDir := filepath.Join(root, b.Name())
		// 只处理 rooms/ 与 cover/ 两个子树，bills/ 归凭证 GC
		for _, sub := range []string{"rooms", "cover"} {
			subRoot := filepath.Join(bDir, sub)
			if _, statErr := os.Stat(subRoot); statErr != nil {
				continue
			}
			d, c, wErr := gcLocalMediaDir(subRoot, filepath.ToSlash(filepath.Join("buildings", b.Name(), sub)),
				referenced, cutoff)
			deleted += d
			checked += c
			if wErr != nil {
				err = wErr
			}
		}
	}
	// 顺带清掉空目录（删完文件后残留的目录树）
	pruneEmptyDirs(root)
	return deleted, checked, err
}

// gcLocalMediaDir 递归遍历单个子树目录，删除未被引用且超过宽限期的文件。
func gcLocalMediaDir(dir, keyPrefix string, referenced map[string]bool, cutoff time.Time) (deleted, checked int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			d, c, subErr := gcLocalMediaDir(full, keyPrefix+"/"+e.Name(), referenced, cutoff)
			deleted += d
			checked += c
			if subErr != nil && err == nil {
				err = subErr
			}
			continue
		}
		checked++
		key := keyPrefix + "/" + e.Name()
		if referenced[key] {
			continue
		}
		info, statErr := e.Info()
		if statErr != nil || info.ModTime().After(cutoff) {
			continue // 宽限期内
		}
		if rmErr := os.Remove(full); rmErr != nil {
			logger.Log.Warn().Err(rmErr).Str("key", key).Msg("媒体GC: 删除本地文件失败")
			continue
		}
		deleted++
	}
	return deleted, checked, nil
}

// pruneEmptyDirs 自底向上删除空目录（保留 buildings/ 根目录本身）。
func pruneEmptyDirs(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		pruneEmptyDirs(dir)
		if sub, err := os.ReadDir(dir); err == nil && len(sub) == 0 {
			_ = os.Remove(dir)
		}
	}
}

// gcQiniuMedia 七牛模式：列举 bucket 下 buildings/ 前缀，只处理 rooms/ 与 cover/ 子树。
func gcQiniuMedia(cfg *config.Config, referenced map[string]bool, cutoff time.Time) (deleted, checked int, err error) {
	keys, err := qiniu.ListKeys(cfg, "buildings/")
	if err != nil {
		return 0, 0, err
	}
	var targets []string
	for _, k := range keys {
		if isMediaKey(k) {
			targets = append(targets, k)
		}
	}
	// 七牛没有「按 key 查修改时间」的批量接口，逐个 Stat 成本高；
	// 这里统一用「对象存储列举结果里的 PutTime」不可得时的保守策略：
	// 未被引用即视为孤儿，但受宽限期保护需要时间戳——用 Stat 逐个取。
	for _, k := range targets {
		checked++
		if referenced[k] {
			continue
		}
		putTime, statErr := qiniu.PutTime(cfg, k)
		if statErr != nil {
			continue
		}
		if putTime.IsZero() || putTime.After(cutoff) {
			continue
		}
		if delErr := qiniu.Delete(cfg, k); delErr != nil {
			logger.Log.Warn().Err(delErr).Str("key", k).Msg("媒体GC: 删除七牛文件失败")
			continue
		}
		deleted++
	}
	return deleted, checked, nil
}
