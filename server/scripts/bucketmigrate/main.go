// bucketmigrate 把七牛旧 bucket 的全部文件**服务端拷贝**到新 bucket（同账号、同区域）。
// 用于七牛免费测试域名 30 天到期后的"换桶"流程：新建 bucket2 拿到新域名后，
// 把旧桶数据整体迁过去，roomsys 改两行配置重启即可无感切换。
//
// 为什么能做到"无感"：
//   - roomsys 数据库里存的只是文件 key（如 buildings/3_xxx/rooms/xxx/images/uuid.jpg），
//     不是完整 URL；bucket 名与 CDN 域名只在 config.json 与运行时配置里（见
//     handlers/media.go，URL 是请求时动态拼的）。因此 key 原样拷贝到新桶后，
//     数据库一行都不用改，只需切换 qiniu_bucket 和 qiniu_domain 两个配置。
//
// 使用场景：
//   - 七牛测试域名到期（30 天，不可续期），新建 bucket 拿新域名时迁移历史文件
//   - 更换正式 CDN 域名/换桶时的数据搬迁
//   注意：旧桶的测试域名是否已过期不影响本脚本——rs 拷贝走服务端内部接口，
//   不经过任何域名，也**不消耗 CDN/下载流量**。
//
// 流程与原理：
//  1. config.Load() 读取七牛 AK/SK 与区域（qiniu_zone；新旧桶必须在同一区域，
//     跨区域 rs 拷贝不支持，需改用 qshell 的 qdownload+qupload 方案）
//  2. 校验源/目标 bucket 都存在于当前账号（防止名字敲错后白跑）
//  3. ListFiles 分页遍历源桶全部文件（每页 1000）
//  4. 按 500 个一批调 Batch(URICopy(...)) 服务端拷贝：
//     code 200=拷贝成功；614=目标已存在（跳过，因此脚本可中断重跑、幂等续传）；
//     其他=失败并打印 key 与原因
//  5. 输出汇总与"迁移后操作"提示
//
// 迁移后操作（脚本结束时也会打印）：
//   1. 停后端 → 改 config.json：qiniu_bucket 改成新桶、qiniu_domain 改成新桶绑定的域名
//   2. 启动后端，图片/视频即从新桶读取
//   3. 旧桶确认无回滚需求后，可在七牛控制台清空以释放存储额度（迁移期间两份数据都占额度）
//   4. rs 拷贝会保留文件的 MIME 与自定义元数据（含 cache-control）；如个别文件元数据
//      异常，重跑 scripts/fixcache 兜底
//
// 用法（在 server 目录下，需读取 config.json）：
//   go run ./scripts/bucketmigrate -dst 新桶名            # 源桶默认取 config.json 的 qiniu_bucket
//   go run ./scripts/bucketmigrate -src 旧桶 -dst 新桶
//   go run ./scripts/bucketmigrate -dst 新桶 -dry         # 只统计不拷贝，预览迁移规模
//   go run ./scripts/bucketmigrate -dst 新桶 -force       # 覆盖目标桶同名文件（默认跳过）
//
// 建议顺序：先 -dry 预览 → 正式拷贝 → 抽查几张图 → 改配置重启。全程可重复执行。
package main

import (
	"flag"
	"fmt"
	"os"

	"rental-server/config"

	qiniuAuth "github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

// batchChunk 每批拷贝的文件数。七牛 rs batch 单请求上限 1000 个操作，取 500 留余量
const batchChunk = 500

// codeExists 目标已存在的 rs 错误码（幂等续传时视为跳过）
const codeExists = 614

func main() {
	var (
		src   = flag.String("src", "", "源 bucket，默认取 config.json 的 qiniu_bucket")
		dst   = flag.String("dst", "", "目标 bucket（必填）")
		force = flag.Bool("force", false, "目标已存在同名文件时覆盖（默认跳过）")
		dry   = flag.Bool("dry", false, "只列出源桶文件统计规模，不执行拷贝")
	)
	flag.Parse()

	cfg := config.Load()
	if cfg.QiniuAccessKey == "" || cfg.QiniuSecretKey == "" {
		fmt.Println("错误：config.json 未配置七牛 AK/SK（qiniu_access_key / qiniu_secret_key）")
		os.Exit(1)
	}
	if *dst == "" {
		fmt.Println("错误：必须用 -dst 指定目标 bucket（如 -dst roomsys2）")
		flag.Usage()
		os.Exit(1)
	}
	if *src == "" {
		*src = cfg.QiniuBucket
		fmt.Printf("未指定 -src，使用 config.json 的 qiniu_bucket: %s\n", *src)
	}
	if *src == *dst {
		fmt.Println("警告：源与目标是同一个 bucket，拷贝会全部以\"目标已存在\"跳过（可用于校验脚本连通性）")
	}

	mac := qiniuAuth.NewMac(cfg.QiniuAccessKey, cfg.QiniuSecretKey)
	bm := storage.NewBucketManager(mac, &storage.Config{Region: getZone(cfg.QiniuZone), UseHTTPS: cfg.QiniuUseHTTPS})

	// 校验两个 bucket 都在当前账号下，避免名字敲错后跑到一半才发现
	buckets, err := bm.Buckets(false)
	if err != nil {
		fmt.Printf("错误：获取账号 bucket 列表失败: %v\n", err)
		os.Exit(1)
	}
	srcOK, dstOK := false, false
	for _, b := range buckets {
		if b == *src {
			srcOK = true
		}
		if b == *dst {
			dstOK = true
		}
	}
	if !srcOK {
		fmt.Printf("错误：源 bucket %q 不在当前账号的空间列表里，检查拼写或 AK 是否正确\n", *src)
		os.Exit(1)
	}
	if !dstOK {
		fmt.Printf("错误：目标 bucket %q 不在当前账号的空间列表里（先在七牛控制台创建，且必须与源桶同区域 %s）\n", *dst, zoneName(cfg.QiniuZone))
		os.Exit(1)
	}

	// 分页遍历源桶全部文件
	fmt.Printf("正在遍历源桶 %s ...\n", *src)
	var keys []string
	var totalSize int64
	marker := ""
	for {
		entries, _, nextMarker, hasNext, err := bm.ListFiles(*src, "", "", marker, 1000)
		if err != nil {
			fmt.Printf("错误：列出文件失败: %v\n", err)
			os.Exit(1)
		}
		for _, e := range entries {
			keys = append(keys, e.Key)
			totalSize += e.Fsize
		}
		if !hasNext || nextMarker == "" {
			break
		}
		marker = nextMarker
	}
	fmt.Printf("源桶共 %d 个文件，合计 %.1f MB\n", len(keys), float64(totalSize)/1024/1024)

	if *dry {
		fmt.Println("dry-run 模式：仅统计，未执行拷贝")
		return
	}

	// 分批服务端拷贝（不耗流量；614 视为已存在跳过，保证可重复执行）
	copied, exists, failed := 0, 0, 0
	for start := 0; start < len(keys); start += batchChunk {
		end := start + batchChunk
		if end > len(keys) {
			end = len(keys)
		}
		ops := make([]string, 0, end-start)
		for _, k := range keys[start:end] {
			ops = append(ops, storage.URICopy(*src, k, *dst, k, *force))
		}
		rets, err := bm.Batch(ops)
		if err != nil {
			// 整批请求失败（网络/鉴权等），计入失败后继续后面的批次
			fmt.Printf("批次 %d-%d 请求失败: %v\n", start, end, err)
			failed += end - start
			continue
		}
		for i, ret := range rets {
			switch ret.Code {
			case 200:
				copied++
			case codeExists:
				exists++
			default:
				failed++
				if failed <= 10 {
					fmt.Printf("拷贝失败 %s: code=%d %s\n", keys[start+i], ret.Code, ret.Data.Error)
				}
			}
		}
		if (start/batchChunk)%10 == 0 {
			fmt.Printf("进度 %d/%d（成功 %d，已存在 %d，失败 %d）\n", end, len(keys), copied, exists, failed)
		}
	}

	fmt.Printf("\n拷贝完成：成功 %d，目标已存在跳过 %d，失败 %d\n", copied, exists, failed)
	if failed > 0 {
		fmt.Println("存在失败项：直接重跑本脚本即可续传（已存在的会自动跳过）")
	}

	fmt.Println(`
—— 迁移后操作 ——
1. 停止后端，修改 config.json：qiniu_bucket 改为 ` + *dst + `，qiniu_domain 改为新桶绑定的域名
2. 启动后端，即从新桶读取图片/视频，数据库无需任何改动
3. 旧桶 ` + *src + ` 保留可作回滚备份；确认无误后在七牛控制台清空以释放免费存储额度`)
}

// getZone 把 config.json 的 qiniu_zone 转为 SDK 区域对象
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

// zoneName 返回区域的展示名，用于错误提示
func zoneName(zone string) string {
	switch zone {
	case "z0":
		return "华东 z0"
	case "z1":
		return "华北 z1"
	case "z2":
		return "华南 z2"
	case "na0":
		return "北美 na0"
	case "as0":
		return "新加坡 as0"
	default:
		return "华南 z2（默认）"
	}
}
