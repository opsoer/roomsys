// qiniudomains 列出当前配置 bucket 在七牛上绑定的域名（用于排查 CDN 域名是否生效）。
//
// 使用场景：
//   - 配置 config.json 的 qiniu_domain 前先确认 bucket 到底绑了哪些域名
//   - 图片/视频加载不出来时排查：bucket 没绑域名 / 绑的域名与配置不一致 /
//     测试域名已过期被七牛回收
//
// 基本原理：读取 config.json（或 CONFIG_PATH）里的七牛 AK/SK/bucket/区域，
// 调 BucketManager.ListBucketDomains 查询并逐行打印域名。
// 七牛测试域名（*.clouddn.com 等）有效期 30 天且不可续期，过期后此命令能看到域名消失，
// 此时媒体文件只能走代理模式或换备案域名。
//
// 运行方式（在 server 目录下）：go run ./scripts/qiniudomains
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