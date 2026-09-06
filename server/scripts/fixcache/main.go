package main

import (
	"fmt"
	"rental-server/config"

	qiniuAuth "github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

func main() {
	cfg := config.Load()
	if cfg.QiniuAccessKey == "" || cfg.QiniuBucket == "" {
		fmt.Println("七牛云未配置，跳过")
		return
	}

	mac := qiniuAuth.NewMac(cfg.QiniuAccessKey, cfg.QiniuSecretKey)
	qiniuCfg := storage.Config{Region: getZone(cfg.QiniuZone), UseHTTPS: cfg.QiniuUseHTTPS}
	bucketMgr := storage.NewBucketManager(mac, &qiniuCfg)

	limit := 1000
	prefix := ""
	marker := ""
	total := 0

	for {
		entries, _, nextMarker, hasNext, err := bucketMgr.ListFiles(cfg.QiniuBucket, prefix, "", marker, limit)
		if err != nil {
			fmt.Printf("列出文件失败: %v\n", err)
			return
		}

		for _, entry := range entries {
			metas := map[string]string{
				"cache-control": "public, max-age=31536000, immutable",
			}
			err := bucketMgr.ChangeMeta(cfg.QiniuBucket, entry.Key, metas)
			if err != nil {
				fmt.Printf("设置 %s 失败: %v\n", entry.Key, err)
			} else {
				total++
				fmt.Printf("[%d] %s\n", total, entry.Key)
			}
		}

		if !hasNext || nextMarker == "" {
			break
		}
		marker = nextMarker
	}

	fmt.Printf("完成，共更新 %d 个文件的 Cache-Control\n", total)
}

func getZone(zone string) *storage.Zone {
	var regionID storage.RegionID
	switch zone {
	case "z0":
		regionID = storage.RIDHuadong
	case "z1":
		regionID = storage.RIDHuabei
	case "z2":
		regionID = storage.RIDHuanan
	case "na0":
		regionID = storage.RIDNorthAmerica
	case "as0":
		regionID = storage.RIDSingapore
	default:
		regionID = storage.RIDHuanan
	}
	z, _ := storage.GetRegionByID(regionID)
	return &z
}
