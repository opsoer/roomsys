# -*- coding: utf-8 -*-
"""fixcache 给七牛 bucket 中所有已存在的对象补写 Cache-Control 响应缓存头
（默认 public, max-age=604800, immutable，时长取配置 storage.media_max_age），跑一次即可。

场景：存量文件一次性修复——早期版本上传的图片/视频没有缓存元数据，CDN 和浏览器每次都
回源；新代码上传已自动带缓存头（handlers/media.go 的 x-qn-meta-cache-control），
本脚本只用于修复之前上传的文件，重复运行无害但没必要。

原理：
  1. 读取 config.yaml（或 CONFIG_PATH）拿七牛 AK/SK/bucket/区域
  2. 分页 ListFiles（每页 1000）遍历 bucket 全部对象
  3. 逐个 ChangeMeta 写入 x-qn-meta-cache-control 并打印进度

注意：必须在 server 目录下运行；未配置七牛时直接跳过。文件名都是 UUID 不会变化，
immutable 策略安全；对象较多时发起等量 ChangeMeta 请求（管理类接口，无额外费用）。

运行方式（在 server 目录下）：python scripts_py/fixcache.py
"""
from __future__ import annotations

from _common import QiniuClient, QiniuError, load_config, media_cache_control


def main():
    cfg = load_config()
    qiniu = cfg.storage.qiniu
    if not qiniu.access_key or not qiniu.bucket:
        print("七牛云未配置，跳过")
        return

    client = QiniuClient(qiniu.access_key, qiniu.secret_key, zone=qiniu.zone, use_https=qiniu.use_https)

    limit = 1000
    prefix = ""
    marker = ""
    total = 0

    while True:
        try:
            entries, next_marker = client.list_files(qiniu.bucket, prefix, "", marker, limit)
        except QiniuError as exc:
            print("列出文件失败: %s" % exc)
            return

        for entry in entries:
            metas = {"cache-control": media_cache_control(cfg)}
            try:
                client.change_meta(qiniu.bucket, entry.get("key", ""), metas)
            except QiniuError as exc:
                print("设置 %s 失败: %s" % (entry.get("key", ""), exc))
            else:
                total += 1
                print("[%d] %s" % (total, entry.get("key", "")))

        if not next_marker:
            break
        marker = next_marker

    print("完成，共更新 %d 个文件的 Cache-Control" % total)


if __name__ == "__main__":
    main()
