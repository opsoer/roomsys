# -*- coding: utf-8 -*-
"""bucketmigrate 把七牛旧 bucket 的全部文件**服务端拷贝**到新 bucket（同账号、同区域）。
用于七牛免费测试域名 30 天到期后的“换桶”流程：新建 bucket2 拿到新域名后把旧桶数据整体
迁过去，roomsys 改两行配置重启即可无感切换。

为什么无感：roomsys 数据库只存文件 key，不存完整 URL；bucket 名与 CDN 域名只在
config.yaml（见 handlers/media.go，URL 请求时动态拼）。key 原样拷到新桶后数据库一行
不用改，只切 storage.qiniu.bucket 与 storage.qiniu.domain 两个配置。

流程：
  1. 读 config.yaml 的七牛 AK/SK 与区域（新旧桶必须同区域，跨区域需 qshell 方案）
  2. 校验源/目标 bucket 都在当前账号（防名字敲错）
  3. ListFiles 分页遍历源桶全部文件（每页 1000）
  4. 按 500 个一批 Batch(URICopy) 服务端拷贝：200=成功；614=目标已存在（跳过，可中断
     重跑、幂等续传）；其他=失败打印 key 与原因
  5. 输出汇总与“迁移后操作”提示

rs 拷贝走服务端内部接口，不经过任何域名、不消耗 CDN/下载流量，保留 MIME 与自定义
元数据（含 cache-control）；个别文件元数据异常可重跑 fixcache 兜底。

用法（在 server 目录下）：
  python scripts_py/bucketmigrate.py -dst 新桶名        # 源桶默认取 config 的 storage.qiniu.bucket
  python scripts_py/bucketmigrate.py -src 旧桶 -dst 新桶
  python scripts_py/bucketmigrate.py -dst 新桶 -dry     # 只统计不拷贝
  python scripts_py/bucketmigrate.py -dst 新桶 -force   # 覆盖目标桶同名文件（默认跳过）
建议顺序：先 -dry 预览 → 正式拷贝 → 抽查 → 改配置重启。全程可重复执行。
"""
from __future__ import annotations

import argparse
import sys

from _common import QiniuClient, QiniuError, load_config, uri_copy

BATCH_CHUNK = 500
CODE_EXISTS = 614


def build_parser():
    parser = argparse.ArgumentParser(
        prog="bucketmigrate",
        description="把七牛旧 bucket 的全部文件服务端拷贝到新 bucket（同账号、同区域）",
    )
    parser.add_argument("-src", dest="src", default="", help="源 bucket，默认取 config.yaml 的 storage.qiniu.bucket")
    parser.add_argument("-dst", dest="dst", default="", help="目标 bucket（必填）")
    parser.add_argument("-force", dest="force", action="store_true", help="目标已存在同名文件时覆盖（默认跳过）")
    parser.add_argument("-dry", dest="dry", action="store_true", help="只列出源桶文件统计规模，不执行拷贝")
    return parser


def main():
    parser = build_parser()
    args = parser.parse_args()
    cfg = load_config()
    qiniu = cfg.storage.qiniu
    if not qiniu.access_key or not qiniu.secret_key:
        print("错误：config.yaml 未配置七牛 AK/SK（storage.qiniu.access_key / storage.qiniu.secret_key）")
        sys.exit(1)
    if not args.dst:
        print("错误：必须用 -dst 指定目标 bucket（如 -dst roomsys2）")
        parser.print_help()
        sys.exit(1)
    src = args.src
    if not src:
        src = qiniu.bucket
        print("未指定 -src，使用 config.yaml 的 storage.qiniu.bucket: %s" % src)
    if src == args.dst:
        print('警告：源与目标是同一个 bucket，拷贝会全部以"目标已存在"跳过（可用于校验脚本连通性）')

    client = QiniuClient(qiniu.access_key, qiniu.secret_key, zone=qiniu.zone, use_https=qiniu.use_https)

    try:
        buckets = client.buckets()
    except QiniuError as exc:
        print("错误：获取账号 bucket 列表失败: %s" % exc)
        sys.exit(1)
    if src not in buckets:
        print('错误：源 bucket "%s" 不在当前账号的空间列表里，检查拼写或 AK 是否正确' % src)
        sys.exit(1)
    if args.dst not in buckets:
        print('错误：目标 bucket "%s" 不在当前账号的空间列表里（先在七牛控制台创建，且必须与源桶同区域 %s）' % (
            args.dst,
            zone_name(qiniu.zone),
        ))
        sys.exit(1)

    print("正在遍历源桶 %s ..." % src)
    keys = []
    total_size = 0
    marker = ""
    while True:
        try:
            entries, next_marker = client.list_files(src, "", "", marker, 1000)
        except QiniuError as exc:
            print("错误：列出文件失败: %s" % exc)
            sys.exit(1)
        for entry in entries:
            keys.append(entry.get("key", ""))
            total_size += int(entry.get("fsize", 0) or 0)
        if not next_marker:
            break
        marker = next_marker
    print("源桶共 %d 个文件，合计 %.1f MB" % (len(keys), total_size / 1024 / 1024))

    if args.dry:
        print("dry-run 模式：仅统计，未执行拷贝")
        return

    copied, exists, failed = 0, 0, 0
    for start in range(0, len(keys), BATCH_CHUNK):
        end = min(start + BATCH_CHUNK, len(keys))
        ops = [uri_copy(src, key, args.dst, key, args.force) for key in keys[start:end]]
        try:
            results = client.batch(ops)
        except QiniuError as exc:
            print("批次 %d-%d 请求失败: %s" % (start, end, exc))
            failed += end - start
            continue
        for index, result in enumerate(results):
            code = result.get("code", 0)
            if code == 200:
                copied += 1
            elif code == CODE_EXISTS:
                exists += 1
            else:
                failed += 1
                if failed <= 10:
                    key = keys[start + index] if start + index < len(keys) else "?"
                    error = result.get("error") or (result.get("data") or {}).get("error") or ""
                    print("拷贝失败 %s: code=%d %s" % (key, code, error))
        if (start // BATCH_CHUNK) % 10 == 0:
            print("进度 %d/%d（成功 %d，已存在 %d，失败 %d）" % (end, len(keys), copied, exists, failed))

    print("\n拷贝完成：成功 %d，目标已存在跳过 %d，失败 %d" % (copied, exists, failed))
    if failed > 0:
        print("存在失败项：直接重跑本脚本即可续传（已存在的会自动跳过）")

    print(
        "\n—— 迁移后操作 ——\n"
        "1. 停止后端，修改 config.yaml：storage.qiniu.bucket 改为 %s，storage.qiniu.domain 改为新桶绑定的域名\n"
        "2. 启动后端，即从新桶读取图片/视频，数据库无需任何改动\n"
        "3. 旧桶 %s 保留可作回滚备份；确认无误后在七牛控制台清空以释放免费存储额度" % (args.dst, src)
    )


def zone_name(zone):
    return {
        "z0": "华东 z0",
        "z1": "华北 z1",
        "z2": "华南 z2",
        "na0": "北美 na0",
        "as0": "新加坡 as0",
    }.get(zone, "华南 z2（默认）")


if __name__ == "__main__":
    main()
