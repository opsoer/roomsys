// fixcache 给七牛 bucket 中所有已存在的对象补写 Cache-Control 响应缓存头
// （public, max-age=31536000, immutable），跑一次即可。
//
// 使用场景：
//   - 存量文件是一次性修复：早期版本上传到七牛的图片/视频没有缓存元数据，
//     CDN 和浏览器每次都回源拉取，白白消耗七牛流量费用。
//     新代码上传时已自动带缓存头（handlers/media.go 的 x-qn-meta-cache-control），
//     因此本脚本只用于给「修复之前」上传的文件补写，重复运行无害但没必要。
//
// 基本原理：
//  1. 读取 config.json（或 CONFIG_PATH）拿到七牛 AK/SK/bucket/区域
//  2. 用 BucketManager.ListFiles 分页（每页 1000）遍历 bucket 内全部对象
//  3. 逐个 ChangeMeta 写入 cache-control 元数据并打印进度
//
// 注意：
//   - 必须在 server 目录下运行（要读 config.json）；未配置七牛时会直接跳过
//   - 文件名都是 UUID，内容不会变化，所以 immutable 缓存策略是安全的
//   - 对象较多时会发起等量次数的 ChangeMeta 请求，属于七牛管理类接口，正常计费无额外费用
//
// 运行方式（在 server 目录下）：go run ./scripts/fixcache
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
