# -*- coding: utf-8 -*-
"""seedmedia 通过 HTTP API 为已有演示数据补充图片：
  - 每个公寓 1 张封面（POST /api/building/cover）
  - 每个房间 1 张照片（POST /api/building/rooms/:id/media）

场景：跑完 seed 后没有图片观感差；按公寓 id / 房间 id 作稳定 seed 从 picsum.photos
拉图，同一 id 永远同图，可重复运行（下载缓存与上传结果不重复处理）。

流程（三阶段）：
  1. root/root 登录，GET /api/admin/buildings 拉全部公寓；逐个用「公寓 id」账号登录，
     GET /api/building/rooms 拉房间（单页最多 100 条）
  2. 16 并发从 picsum.photos 下载（封面 1200x800 / 房间图 800x600）
  3. 8 并发上传：每公寓先登录拿 token，再依次传封面 + 房间图

限流：脚本自限速 200 次/分钟（仅约束对后端的请求，下载图片不受影响），429 自动等
窗口重置重试。环境变量：SEED_BASE_URL（默认 http://127.0.0.1:8081）、SEED_TMP_DIR
（默认 <系统临时目录>/seedmedia）、SEED_RATE_PER_MIN（默认 200，可设 0 关闭）。

前置：后端已运行且已跑过 scripts/seed；楼栋管理员账号存在（用户名=密码=公寓 id）；
机器能访问 picsum.photos（外网）。

运行方式（在 server 目录下）：python scripts_py/seedmedia.py
预计耗时：5000+ 张图在默认自限速下约 25~30 分钟（大头是上传）。
"""
from __future__ import annotations

import os
import tempfile
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from urllib.error import HTTPError
from urllib.request import urlopen

from _common import die, do_json, do_upload, login

DOWNLOAD_WORKERS = 16
UPLOAD_WORKERS = 8
ROOM_IMAGE_W = 800
ROOM_IMAGE_H = 600
COVER_W = 1200
COVER_H = 800

TMP_DIR = os.environ.get("SEED_TMP_DIR") or os.path.join(tempfile.gettempdir(), "seedmedia")

_download_counter = [0]
_upload_counter = [0]
_fail_counter = [0]
_total_downloads = [0]
_counter_lock = threading.Lock()
_tokens = {}
_tokens_lock = threading.Lock()


def main():
    try:
        os.makedirs(TMP_DIR, exist_ok=True)
    except OSError as exc:
        die(str(exc))

    try:
        root_token = login("root", "root")
    except Exception as exc:
        die("超级管理员登录失败：%s" % exc)

    try:
        list_data = do_json("GET", "/api/admin/buildings?page=1&page_size=100", root_token)
    except Exception as exc:
        die("获取公寓列表失败：%s" % exc)

    buildings = []
    for item in (list_data or {}).get("buildings") or []:
        building_id = item.get("id", 0)
        username = str(building_id)
        try:
            token = login(username, username)
        except Exception as exc:
            die("楼栋管理员 %s 登录失败：%s" % (username, exc))
        try:
            rooms = do_json("GET", "/api/building/rooms?page=1&page_size=100", token)
        except Exception as exc:
            die("获取公寓 %d 房间列表失败：%s" % (building_id, exc))
        room_ids = [room.get("id", 0) for room in ((rooms or {}).get("rooms") or [])]
        buildings.append({"id": building_id, "rooms": room_ids})
    print("共 %d 个公寓，%d 个房间" % (len(buildings), total_rooms(buildings)))

    start = time.monotonic()
    download_images(buildings)
    print("图片下载完成，耗时 %s" % fmt_duration(time.monotonic() - start))

    start = time.monotonic()
    upload_images(buildings, TMP_DIR)
    print("上传完成：%d 张，耗时 %s" % (_upload_counter[0], fmt_duration(time.monotonic() - start)))


def total_rooms(buildings):
    return sum(len(item["rooms"]) for item in buildings)


def fmt_duration(seconds):
    total = int(seconds + 0.5)
    hours, remainder = divmod(total, 3600)
    minutes, secs = divmod(remainder, 60)
    if hours:
        return "%dh%dm%ds" % (hours, minutes, secs)
    if minutes:
        return "%dm%ds" % (minutes, secs)
    return "%ds" % secs


def download(seed_key, width, height, out_dir):
    path = os.path.join(out_dir, seed_key + ".jpg")
    if os.path.isfile(path):
        return path, None
    url = "https://picsum.photos/seed/%s/%d/%d" % (seed_key, width, height)

    last_error = None
    for attempt in range(4):
        if attempt:
            time.sleep(attempt * 0.8)
        status = None
        data = None
        try:
            with urlopen(url) as response:
                status = response.status
                data = response.read()
        except HTTPError as exc:
            status = exc.code
            try:
                data = exc.read()
            except Exception as exc2:
                last_error = exc2
                continue
        except Exception as exc:
            last_error = exc
            continue
        if status != 200 or len(data) < 1024:
            continue
        try:
            with open(path, "wb") as fh:
                fh.write(data)
        except OSError as exc:
            return "", exc
        return path, None
    return "", last_error


def download_task(seed_key, width, height):
    path, error = download(seed_key, width, height, TMP_DIR)
    if error is not None:
        print("下载失败 %s: %s" % (seed_key, error))
        return
    with _counter_lock:
        _download_counter[0] += 1
        count = _download_counter[0]
    if count % 500 == 0:
        print("已下载 %d/%d" % (count, _total_downloads[0]))


def download_images(buildings):
    tasks = []
    for building in buildings:
        tasks.append(("b%03d" % building["id"], COVER_W, COVER_H))
        for room_id in building["rooms"]:
            tasks.append(("r%06d" % room_id, ROOM_IMAGE_W, ROOM_IMAGE_H))
    _total_downloads[0] = len(tasks)
    _download_counter[0] = 0

    with ThreadPoolExecutor(max_workers=DOWNLOAD_WORKERS) as pool:
        futures = [pool.submit(download_task, seed_key, width, height) for seed_key, width, height in tasks]
        for future in futures:
            future.result()
    print("已下载 %d/%d" % (_download_counter[0], len(tasks)))


def upload_building(building, out_dir):
    with _tokens_lock:
        token = _tokens.get(building["id"])
        if token is None:
            username = str(building["id"])
            try:
                token = login(username, username)
            except Exception as exc:
                print("楼栋管理员 %s 登录失败：%s" % (username, exc))
                return
            _tokens[building["id"]] = token

    cover_path = os.path.join(out_dir, "b%03d" % building["id"] + ".jpg")
    try:
        with open(cover_path, "rb") as fh:
            data = fh.read()
        do_upload("/api/building/cover", token, "file", "cover_%d.jpg" % building["id"], data)
    except Exception as exc:
        print("公寓 %d 封面上传失败：%s" % (building["id"], exc))
    else:
        with _counter_lock:
            _upload_counter[0] += 1

    for room_id in building["rooms"]:
        image_path = os.path.join(out_dir, "r%06d" % room_id + ".jpg")
        try:
            with open(image_path, "rb") as fh:
                data = fh.read()
            do_upload(
                "/api/building/rooms/%d/media" % room_id,
                token,
                "file",
                "room_%d.jpg" % room_id,
                data,
                {"category": "gallery"},
            )
        except Exception as exc:
            with _counter_lock:
                _fail_counter[0] += 1
                count = _fail_counter[0]
            if count <= 10:
                print("房间 %d 图片上传失败：%s" % (room_id, exc))
            continue
        with _counter_lock:
            _upload_counter[0] += 1
            count = _upload_counter[0]
        if count % 500 == 0:
            print("已上传 %d 张" % count)


def upload_images(buildings, out_dir):
    _upload_counter[0] = 0
    _fail_counter[0] = 0
    with ThreadPoolExecutor(max_workers=UPLOAD_WORKERS) as pool:
        futures = [pool.submit(upload_building, building, out_dir) for building in buildings]
        for future in futures:
            future.result()
    if _fail_counter[0] > 0:
        print("共 %d 张上传失败" % _fail_counter[0])


if __name__ == "__main__":
    main()
