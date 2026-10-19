# -*- coding: utf-8 -*-
"""resetdb 清空本地数据库：删除当前库中的全部表（包括 root 账号所在的 users 表）。

清完后无需手工做事——下次启动后端时启动流程的 AutoMigrate 会重建全部表并自动创建
超级管理员 root/root，得到与全新部署等效的干净环境。支持 config.yaml 的
database.driver 双后端（mysql / sqlite）。

流程：config 加载（须在 server 目录运行）→ 动态查出全部表（information_schema /
sqlite_master，不硬编码模型清单）→ 绕开外键逐表 DROP（MySQL SET FOREIGN_KEY_CHECKS=0；
SQLite 固定单连接 PRAGMA foreign_keys=OFF，清完 wal_checkpoint）→ 只删表不动库本身。

安全机制：默认交互确认（输入 yes 才执行），-y/--yes 跳过；执行前打印目标实例与表数量。
参数：-y/--yes 跳过确认；--media 额外清空本地媒体目录 server.upload_dir
（七牛云端对象不受影响，清云端请到七牛控制台操作）。

运行方式（在 server 目录下）：
  python scripts_py/resetdb.py              # 交互确认后清库
  python scripts_py/resetdb.py -y           # 跳过确认
  python scripts_py/resetdb.py -y --media   # 连本地媒体文件一起清
建议流程：先停后端 → 清库 → 启动后端（重建表 + root/root）→ 需要演示数据再跑 seed。
"""
from __future__ import annotations

import os
import shutil
import sys

from _common import die, load_config, open_database


def main():
    yes = False
    clean_media = False
    for arg in sys.argv[1:]:
        if arg in ("-y", "--yes"):
            yes = True
        elif arg == "--media":
            clean_media = True

    cfg = load_config()
    try:
        db = open_database(cfg)
    except Exception as exc:
        die("连接数据库失败: %s" % exc)

    try:
        tables = list_tables(db, cfg)
    except Exception as exc:
        die("查询表清单失败: %s" % exc)
    if not tables:
        print("数据库已是空的，无需清理")
        return

    if cfg.database.driver == "sqlite":
        print("目标：SQLite 文件 %s" % cfg.database.sqlite.path)
    else:
        mysql = cfg.database.mysql
        print("目标：%s:%s / 库 %s" % (mysql.host, mysql.port, mysql.name))
    print("即将删除 %d 张表（含 root 账号），此操作不可恢复：" % len(tables))
    for index, table in enumerate(tables):
        print("  " + table, end="")
        if (index + 1) % 5 == 0 or index == len(tables) - 1:
            print()

    if not yes:
        print("\n确认请输入 yes：", end="")
        sys.stdout.flush()
        line = sys.stdin.readline()
        if line.strip() != "yes":
            print("已取消")
            return

    if cfg.database.driver == "sqlite":
        dropped, err = drop_tables_sqlite(db, tables)
    else:
        dropped, err = drop_tables_mysql(db, tables)
    if err is not None:
        die(str(err))
    print("已删除 %d/%d 张表" % (dropped, len(tables)))

    if clean_media:
        upload_dir = cfg.server.upload_dir
        try:
            remove_all(upload_dir)
        except OSError as exc:
            print("清空媒体目录 %s 失败: %s" % (upload_dir, exc))
        else:
            print("已清空本地媒体目录 %s（七牛云端文件不受影响）" % upload_dir)

    print("\n完成。启动后端进程即可：表结构会由 AutoMigrate 自动重建，root/root 自动创建。")


def remove_all(path):
    if os.path.isdir(path) and not os.path.islink(path):
        shutil.rmtree(path)
    elif os.path.lexists(path):
        os.remove(path)


def list_tables(db, cfg):
    if cfg.database.driver == "sqlite":
        return list_tables_sqlite(db)
    return list_tables_mysql(db, cfg.database.mysql.name)


def drop_tables_mysql(db, tables):
    try:
        with db.cursor() as cursor:
            cursor.execute("SET FOREIGN_KEY_CHECKS=0")
    except Exception as exc:
        return 0, RuntimeError("关闭外键检查失败: %s" % exc)
    dropped = 0
    for table in tables:
        try:
            with db.cursor() as cursor:
                cursor.execute("DROP TABLE IF EXISTS `%s`" % table)
        except Exception as exc:
            print("删除表 %s 失败: %s" % (table, exc))
            continue
        dropped += 1
    try:
        with db.cursor() as cursor:
            cursor.execute("SET FOREIGN_KEY_CHECKS=1")
    except Exception:
        pass
    return dropped, None


def drop_tables_sqlite(db, tables):
    try:
        db.execute("PRAGMA foreign_keys=OFF")
    except Exception as exc:
        return 0, RuntimeError("关闭外键检查失败: %s" % exc)
    dropped = 0
    for table in tables:
        try:
            db.execute('DROP TABLE IF EXISTS "%s"' % table)
        except Exception as exc:
            print("删除表 %s 失败: %s" % (table, exc))
            continue
        dropped += 1
    try:
        db.execute("PRAGMA foreign_keys=ON")
    except Exception:
        pass
    try:
        db.execute("PRAGMA wal_checkpoint(TRUNCATE)")
        db.commit()
    except Exception:
        pass
    return dropped, None


def list_tables_mysql(db, schema):
    with db.cursor() as cursor:
        cursor.execute(
            "SELECT table_name FROM information_schema.tables WHERE table_schema = %s AND table_type = 'BASE TABLE'",
            (schema,),
        )
        rows = cursor.fetchall()
    return [row[0] for row in rows]


def list_tables_sqlite(db):
    cursor = db.execute(
        "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'"
    )
    rows = cursor.fetchall()
    cursor.close()
    return [row[0] for row in rows]


if __name__ == "__main__":
    main()
