# -*- coding: utf-8 -*-
"""qiniudomains 列出当前配置 bucket 在七牛上绑定的域名（排查 CDN 域名是否生效）。

场景：
  - 配置 config.yaml 的 storage.qiniu.domain 前先确认 bucket 绑了哪些域名
  - 图片/视频加载不出来时排查：没绑域名 / 绑的与配置不一致 / 测试域名过期被回收

原理：读取 config.yaml（或 CONFIG_PATH）的七牛 AK/SK/bucket/区域，调
ListBucketDomains（GET /v3/domains）逐行打印域名。七牛测试域名（*.clouddn.com 等）
有效期 30 天不可续期，过期后此命令能看到域名消失，此时媒体只能走代理模式或换备案域名。
与 Go 版一致：固定 use_https=false（以 http 访问七牛管理接口）。

运行方式（在 server 目录下）：python scripts_py/qiniudomains.py
"""
from __future__ import annotations

from _common import QiniuClient, QiniuError, load_config


def main():
    cfg = load_config()
    qiniu = cfg.storage.qiniu
    client = QiniuClient(qiniu.access_key, qiniu.secret_key, zone=qiniu.zone, use_https=False)

    print("bucket: %s" % qiniu.bucket)
    try:
        domains = client.list_domains(qiniu.bucket)
    except QiniuError as exc:
        print("查询域名失败: %s" % exc)
        return
    if not domains:
        print("该 bucket 未绑定任何域名")
        return
    for domain in domains:
        print("域名: %s" % domain)


if __name__ == "__main__":
    main()
