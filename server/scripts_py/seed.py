# -*- coding: utf-8 -*-
"""seed 通过 HTTP API 生成演示数据：100 个公寓（含房东）+ 每栋 50 套房 + 约 70% 签约出租。
所有操作（创建公寓、建楼栋管理员、登录、创建房间、出租签约）全走真实 HTTP 接口，
数据与手工在管理后台录入完全一致（含账单、待办等派生数据）。

场景：本地/演示环境灌入大批量逼真数据，验证分页/地图筛选/统计图表；配合 seedmedia
补图片。注意：会写约 100 公寓 + 5000 房间 + 数千账单，只可用于可丢弃的数据库，
严禁对生产库运行。

流程：root/root 登录 → 循环随机生成 100 个公寓（深圳各区/街道/小区去重，
POST /api/admin/buildings，全套餐 full、contract_date=当天）→ GET 拉回全部公寓 →
每栋 POST /api/admin/auth/create-building-admin 建管理员（用户名=密码=公寓 id）并登录 →
每栋 50 套房（25 单间 / 13 一室一厅 / 12 两房，随机楼层房号）→ 按 70% 概率
PUT /api/building/rooms/:id/status 设为 rented（后端自动生成租客/合同/账单）。

限流：服务端全局 IP 限流默认 240 次/分钟，全速约 9000 请求必撞 429。脚本自限速
200 次/分钟，收到 429 自动等窗口重置重试，无需人工干预。

环境变量：SEED_BASE_URL（默认 http://127.0.0.1:8081）、SEED_RATE_PER_MIN（默认 200，
后端已调大 RATE_LIMIT_PER_MIN 时可设 0 关闭自限速）。

前置：后端已在 SEED_BASE_URL 运行、数据库已清空（要求公寓名单恰好为空）、已有 root/root。

运行方式（在 server 目录下）：python scripts_py/seed.py
预计耗时：默认自限速约 45 分钟；关闭自限速并调大服务端限流约 3~5 分钟。
"""
from __future__ import annotations

import calendar
import random
from datetime import date, timedelta

from _common import ApiError, die, do_json, login

rng = random.Random()

ROOMS_PER_BUILDING = 50
STUDIO_COUNT = 25
ONE_BED_COUNT = 13
TWO_BED_COUNT = 12
RENT_PROBABILITY = 0.7

DISTRICTS = [
    ("福田区", ["华强北街道", "梅林街道", "香蜜湖街道", "车公庙", "福田保税区", "竹子林", "岗厦", "下梅林"]),
    ("南山区", ["科技园", "后海", "前海", "蛇口", "西丽", "桃源", "白石洲", "深圳湾"]),
    ("罗湖区", ["东门", "笋岗", "翠竹", "莲塘", "黄贝岭", "水贝", "泥岗", "布心"]),
    ("宝安区", ["西乡", "新安", "福永", "沙井", "松岗", "石岩", "航城", "福海"]),
    ("龙岗区", ["坂田", "布吉", "龙城", "横岗", "南湾", "平湖", "坪地", "宝龙"]),
    ("龙华区", ["民治", "大浪", "观澜", "观湖", "福城", "梅龙"]),
    ("盐田区", ["沙头角", "海山", "梅沙", "盐田港"]),
    ("光明区", ["光明", "公明", "新湖", "凤凰", "马田"]),
    ("坪山区", ["坪山", "坑梓", "龙田", "马峦"]),
    ("大鹏新区", ["大鹏", "葵涌", "南澳"]),
]

VILLAGE_PREFIXES = [
    "阳光", "翠湖", "金鹏", "燕山", "锦绣", "碧海", "蓝湾", "云顶", "星河", "梧桐",
    "桂花", "荔枝", "梅园", "椰风", "海韵", "凤凰", "龙腾", "紫荆", "兰花", "松柏",
]
VILLAGE_SUFFIXES = ["花园", "家园", "名苑", "雅苑", "豪庭"]

SURNAMES = [
    "张", "王", "李", "赵", "刘", "陈", "杨", "黄", "周", "吴", "徐", "孙", "胡", "朱", "高", "林",
    "何", "郭", "马", "罗", "梁", "宋", "郑", "谢", "韩", "唐", "冯", "于", "董", "萧", "程", "曹",
    "袁", "邓", "许", "傅", "沈", "曾", "彭", "吕", "苏", "卢", "蒋", "蔡", "贾", "丁", "魏", "薛",
]
GIVEN_NAMES = [
    "伟", "芳", "娜", "敏", "静", "磊", "军", "洋", "勇", "艳", "杰", "涛", "明", "超", "霞", "平",
    "刚", "建华", "文", "辉", "力", "鹏", "玉华", "海燕", "志强", "国强", "晓东", "宇", "诚", "浩", "光", "婷",
    "雪", "鑫", "欣", "怡", "雨", "睿", "晨", "阳", "佳", "琪", "淑华", "丽华", "美玲", "秀兰",
]

NOW = date.today()
USED_PHONE = set()


def main():
    print("登录超级管理员 root ...")
    try:
        root_token = login("root", "root")
    except Exception as exc:
        die("超级管理员登录失败（请先启动后端并确认 root/root 可登录）：%s" % exc)
    print("超级管理员登录成功")

    seed_buildings(root_token)
    seed_rooms(root_token)
    print("\n全部完成！楼栋管理员账号：用户名=公寓id，密码同用户名（如公寓 3 → 3 / 3）")


def seed_buildings(root_token):
    used_addr = set()
    used_name = set()

    buildings = 0
    while buildings < 100:
        name_dist, streets = DISTRICTS[rng.randrange(len(DISTRICTS))]
        street = streets[rng.randrange(len(streets))]
        village = VILLAGE_PREFIXES[rng.randrange(len(VILLAGE_PREFIXES))] + VILLAGE_SUFFIXES[rng.randrange(len(VILLAGE_SUFFIXES))]
        building_no = "%d栋" % (1 + rng.randrange(20))

        addr_key = name_dist + street + village + building_no
        if addr_key in used_addr:
            continue
        name = village + building_no
        if name in used_name:
            continue
        used_addr.add(addr_key)
        used_name.add(name)

        landlord_count = 1 + rng.randrange(2)
        landlords = [{"name": random_name(), "phone": random_phone()} for _ in range(landlord_count)]

        body = {
            "name": name,
            "package": "full",
            "contract_date": NOW.strftime("%Y-%m-%d"),
            "district": name_dist,
            "street": street,
            "village": village,
            "building_no": building_no,
            "description": "精装修公寓，拎包入住，周边配套齐全，交通便利，直连房东无中介费。",
            "landlords": landlords,
        }
        try:
            data = do_json("POST", "/api/admin/buildings", root_token, body)
        except Exception as exc:
            die("创建公寓 %s 失败：%s" % (name, exc))
        buildings += 1
        building_id = ((data or {}).get("building") or {}).get("id", 0)
        print("已创建公寓 %d/100：%s（id=%d，房东%d人）" % (buildings, name, building_id, landlord_count))


def seed_rooms(root_token):
    try:
        list_data = do_json("GET", "/api/admin/buildings?page=1&page_size=100", root_token)
    except Exception as exc:
        die("获取公寓列表失败：%s" % exc)
    list_data = list_data or {}
    total = list_data.get("total", 0)
    if total != 100:
        die("期望 100 个公寓，实际 %d 个（请先清空数据库再运行）" % total)

    for idx, building in enumerate(list_data.get("buildings") or []):
        building_id = building.get("id", 0)
        username = str(building_id)
        try:
            do_json(
                "POST",
                "/api/admin/auth/create-building-admin",
                root_token,
                {"username": username, "password": username, "building_id": building_id},
            )
        except ApiError as exc:
            if exc.status == 409:
                print("公寓 %d 的管理员 %s 已存在，跳过创建" % (building_id, username))
            else:
                die("创建公寓 %d 的管理员失败：%s" % (building_id, exc))
        except Exception as exc:
            die("创建公寓 %d 的管理员失败：%s" % (building_id, exc))

        try:
            token = login(username, username)
        except Exception as exc:
            die("楼栋管理员 %s 登录失败：%s" % (username, exc))

        layouts = ["单间"] * STUDIO_COUNT + ["一室一厅"] * ONE_BED_COUNT + ["两房"] * TWO_BED_COUNT
        rng.shuffle(layouts)

        room_ids = []
        room_infos = []
        unit_per_floor = {}
        for i in range(ROOMS_PER_BUILDING):
            layout = layouts[i]
            floor = 1 + rng.randrange(10)
            unit_per_floor[floor] = unit_per_floor.get(floor, 0) + 1
            room_no = "%d%02d" % (floor, unit_per_floor[floor])

            rent_price = rent_for_layout(layout)
            deposit_months = 1 + rng.randrange(2)
            room = {
                "room_number": room_no,
                "floor": str(floor),
                "layout": layout,
                "description": "精装修，家电齐全，拎包入住。",
                "rent_price": rent_price,
                "deposit_months": deposit_months,
                "management_fee": round1(50 + rng.random() * 150),
                "electricity_unit_price": round2(1.2 + rng.random() * 0.6),
                "water_unit_price": round2(4.5 + rng.random() * 2.0),
            }

            try:
                data = do_json("POST", "/api/building/rooms", token, room)
            except Exception as exc:
                die("公寓 %d 创建房间 %s 失败：%s" % (building_id, room_no, exc))
            room_ids.append(((data or {}).get("room") or {}).get("id", 0))
            room_infos.append(room)

        rented = 0
        for i in range(ROOMS_PER_BUILDING):
            if rng.random() >= RENT_PROBABILITY:
                continue
            info = room_infos[i]
            rent_price = info["rent_price"]
            deposit_months = info["deposit_months"]
            management_fee = info["management_fee"]

            start = random_day_in_month(NOW.year, NOW.month)
            end = add_months(start, 1 + rng.randrange(3)) + timedelta(days=rng.randrange(28))

            body = {
                "status": "rented",
                "tenant_name": random_name(),
                "tenant_phone": random_phone(),
                "rent_price": rent_price,
                "management_fee": management_fee,
                "deposit": round1(rent_price * deposit_months),
                "earnest_money": 0,
                "start_date": start.strftime("%Y-%m-%d"),
                "end_date": end.strftime("%Y-%m-%d"),
            }
            try:
                do_json("PUT", "/api/building/rooms/%d/status" % room_ids[i], token, body)
            except Exception as exc:
                die("公寓 %d 出租房间 %s 失败：%s" % (building_id, info["room_number"], exc))
            rented += 1

        print("公寓 %d/100（%s）：50 套房已创建，其中 %d 套已出租签约" % (idx + 1, building.get("name", ""), rented))


def random_day_in_month(year, month):
    days_in_month = calendar.monthrange(year, month)[1]
    return date(year, month, 1 + rng.randrange(days_in_month))


def add_months(day, months):
    total = day.month - 1 + months
    year = day.year + total // 12
    month = total % 12 + 1
    max_day = calendar.monthrange(year, month)[1]
    if day.day > max_day:
        return date(year, month, max_day) + timedelta(days=day.day - max_day)
    return date(year, month, day.day)


def rent_for_layout(layout):
    if layout == "单间":
        minimum, maximum = 1000, 2500
    elif layout == "一室一厅":
        minimum, maximum = 2600, 3800
    else:
        minimum, maximum = 3800, 5000
    steps = int((maximum - minimum) / 100) + 1
    return float(minimum + rng.randrange(steps) * 100)


def random_name():
    return SURNAMES[rng.randrange(len(SURNAMES))] + GIVEN_NAMES[rng.randrange(len(GIVEN_NAMES))] + GIVEN_NAMES[rng.randrange(len(GIVEN_NAMES))]


def random_phone():
    while True:
        phone = "13" + str(rng.randrange(8)) + "%08d" % rng.randrange(100000000)
        if phone not in USED_PHONE:
            USED_PHONE.add(phone)
            return phone


def round1(value):
    return int(value * 10 + 0.5) / 10


def round2(value):
    return int(value * 100 + 0.5) / 100


if __name__ == "__main__":
    main()
