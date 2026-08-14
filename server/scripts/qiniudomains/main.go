// qiniudomains 列出当前配置 bucket 在七牛上绑定的域名（用于排查 CDN 域名是否生效）
package main

import (
	"fmt"

	"rental-server/config"

	qiniuAuth "github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

func main() {
	cfg := config.Load()
	mac := qiniuAuth.NewMac(cfg.QiniuAccessKey, cfg.QiniuSecretKey)
	zone := getZone(cfg.QiniuZone)
	bm := storage.NewBucketManager(mac, &storage.Config{Region: zone, UseHTTPS: false})

	fmt.Printf("bucket: %s\n", cfg.QiniuBucket)
	domains, err := bm.ListBucketDomains(cfg.QiniuBucket)
	if err != nil {
		fmt.Printf("查询域名失败: %v\n", err)
		return
	}
	if len(domains) == 0 {
		fmt.Println("该 bucket 未绑定任何域名")
		return
	}
	for _, d := range domains {
		fmt.Printf("域名: %s\n", d.Domain)
	}
}

func getZone(name string) *storage.Region {
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
		return &storage.ZoneHuanan
	}
}