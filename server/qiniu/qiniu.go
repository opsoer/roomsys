// Package qiniu 提供七牛对象存储的包级操作（上传/列目录/删除），
// 供定时备份等非媒体链路使用；媒体上传仍走 handlers.MediaHandler。
package qiniu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"rental-server/config"

	qiniuAuth "github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

// uploadTimeout 单次上传超时，防止网络异常时备份任务无限阻塞。
const uploadTimeout = 5 * time.Minute

// zoneRegion 与 handlers.getZone 保持一致的区域映射。
func zoneRegion(name string) *storage.Region {
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

// Enabled 判断是否配置了七牛云存储。
func Enabled(cfg *config.Config) bool {
	q := cfg.Storage.Qiniu
	return q.AccessKey != "" && q.SecretKey != "" && q.Bucket != ""
}

func newMac(cfg *config.Config) *qiniuAuth.Mac {
	return qiniuAuth.NewMac(cfg.Storage.Qiniu.AccessKey, cfg.Storage.Qiniu.SecretKey)
}

func newConfig(cfg *config.Config) storage.Config {
	return storage.Config{Region: zoneRegion(cfg.Storage.Qiniu.Zone), UseHTTPS: cfg.Storage.Qiniu.UseHTTPS}
}

// Upload 上传文件到七牛（分片上传，兼容大文件），key 由调用方指定。
func Upload(cfg *config.Config, key string, reader io.ReaderAt, size int64) error {
	putPolicy := storage.PutPolicy{Scope: cfg.Storage.Qiniu.Bucket, Expires: 3600}
	upToken := putPolicy.UploadToken(newMac(cfg))
	sdkCfg := newConfig(cfg)
	uploader := storage.NewResumeUploader(&sdkCfg)
	ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
	defer cancel()
	ret := storage.PutRet{}
	return uploader.Put(ctx, &ret, upToken, key, reader, size, &storage.RputExtra{})
}

// UploadBytes 上传内存中的文件（备份 gzip 产物等小文件）。
func UploadBytes(cfg *config.Config, key string, data []byte) error {
	return Upload(cfg, key, bytes.NewReader(data), int64(len(data)))
}

// ListKeys 列出 bucket 中指定前缀的所有对象 key。
func ListKeys(cfg *config.Config, prefix string) ([]string, error) {
	bm := storage.NewBucketManager(newMac(cfg), ptr(newConfig(cfg)))
	var keys []string
	marker := ""
	for {
		entries, _, nextMarker, hasNext, err := bm.ListFiles(cfg.Storage.Qiniu.Bucket, prefix, "", marker, 1000)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			keys = append(keys, e.Key)
		}
		if !hasNext || nextMarker == "" {
			break
		}
		marker = nextMarker
	}
	return keys, nil
}

// Delete 逐个删除指定 key，返回错误汇总（空列表直接返回 nil）。
func Delete(cfg *config.Config, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	bm := storage.NewBucketManager(newMac(cfg), ptr(newConfig(cfg)))
	var errs []string
	for _, k := range keys {
		if err := bm.Delete(cfg.Storage.Qiniu.Bucket, k); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", k, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("删除失败: %v", errs)
	}
	return nil
}

func ptr[T any](v T) *T { return &v }
