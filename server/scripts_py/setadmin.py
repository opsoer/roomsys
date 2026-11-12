# -*- coding: utf-8 -*-
"""setadmin 直接修改数据库中超级管理员的账号密码（不经 HTTP 接口），部署服务器上就地换凭据。

流程：config 加载（须在 server 目录运行）→ 直连数据库（sqlite / mysql 双后端）→
按原账号查超级管理员（role=super_admin 且未删除）→ bcrypt 校验原密码 → 校验新密码
与确认密码一致、新账号未被其他用户占用（含软删除占位，与 username 唯一索引一致）→
写入新账号与 bcrypt 新哈希，立即生效，无需重启后端。与 Go 版 scripts/setadmin 行为一致。

★ 改完务必同步 config.yaml 的 admin.username / admin.password（或 ADMIN_USERNAME /
ADMIN_PASSWORD 环境变量）：后端每次启动都会把超管账号密码与配置对齐（有则改、无则建），
不同步则下次重启会被改回配置里的旧值。已签发的登录令牌在有效期内仍然可用。

运行方式（在 server 目录下）：
  python scripts_py/setadmin.py <原账号> <原密码> <新账号> <新密码> <确认密码>
示例：
  python scripts_py/setadmin.py root root admin NewPass@123 NewPass@123
"""
from __future__ import annotations

import argparse
from datetime import datetime

from _common import die, load_config, open_database

EPILOG = """\
配置与运行前提：
  1. 在 server 目录下运行（脚本按当前工作目录查找 config.yaml）；
     也可用环境变量 CONFIG_PATH 指定配置文件路径。
  2. 配置加载顺序与后端一致：内置默认值 → config.yaml → 环境变量覆盖
     （DB_DRIVER、DB_PATH、ADMIN_USERNAME、ADMIN_PASSWORD 等）。
  3. 依赖：pip install pyyaml bcrypt（MySQL 还需 pymysql）。
  4. 支持 database.driver = sqlite / mysql；目标实例会在执行前打印。
  5. 密码哈希用 bcrypt（与后端登录校验同一算法），依赖 bcrypt 库。

★ 改完务必同步 config.yaml 的 admin.username / admin.password 为新值，
  否则下次重启后端会把超级管理员的账号密码对齐回配置文件里的旧值。

示例（在 server 目录下）：
  python scripts_py/setadmin.py root root admin NewPass@123 NewPass@123
"""


def build_parser():
    parser = argparse.ArgumentParser(
        prog="setadmin",
        description="直接修改数据库中超级管理员的账号密码（支持 sqlite / mysql），改完需同步 config.yaml 的 admin 段。",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=EPILOG,
    )
    parser.add_argument("old_username", help="原超级管理员账号")
    parser.add_argument("old_password", help="原密码（用于校验身份）")
    parser.add_argument("new_username", help="新账号")
    parser.add_argument("new_password", help="新密码")
    parser.add_argument("new_confirm", help="确认密码（须与新密码一致）")
    return parser


def main():
    args = build_parser().parse_args()
    if not args.old_username or not args.old_password or not args.new_username or not args.new_password:
        die("账号与密码均不能为空")
    if args.new_password != args.new_confirm:
        die("新密码与确认密码不一致")

    try:
        import bcrypt
    except ImportError:
        die("缺少 bcrypt 依赖，请先执行: pip install bcrypt")

    cfg = load_config()
    if cfg.database.driver == "sqlite":
        print("目标：SQLite 文件 %s" % cfg.database.sqlite.path)
    else:
        mysql = cfg.database.mysql
        print("目标：%s:%s / 库 %s" % (mysql.host, mysql.port, mysql.name))
    try:
        db = open_database(cfg)
    except Exception as exc:
        die("连接数据库失败: %s" % exc)

    admin = fetchone(
        db,
        cfg,
        "SELECT id, username, password_hash, is_active FROM users"
        " WHERE username = {ph} AND role = 'super_admin' AND deleted_at IS NULL",
        (args.old_username,),
    )
    if admin is None:
        die("原账号 %r 不存在或不是超级管理员" % args.old_username)
    admin_id, username, password_hash, is_active = admin

    if not bcrypt.checkpw(args.old_password.encode("utf-8"), password_hash.encode("utf-8")):
        die("原密码错误")
    if not is_active:
        print("警告: 该账号当前处于停用状态（is_active=0），改完密码仍无法登录，请先在管理后台启用")

    if args.new_username != username:
        conflict = fetchone(db, cfg, "SELECT id FROM users WHERE username = {ph}", (args.new_username,))
        if conflict is not None and conflict[0] != admin_id:
            die("新账号 %r 已被其他用户占用" % args.new_username)

    new_hash = bcrypt.hashpw(args.new_password.encode("utf-8"), bcrypt.gensalt()).decode("ascii")
    if cfg.database.driver == "mysql":
        execute(
            db, cfg,
            "UPDATE users SET username = {ph}, password_hash = {ph}, updated_at = NOW() WHERE id = {ph}",
            (args.new_username, new_hash, admin_id),
        )
    else:
        now = datetime.now().astimezone().isoformat(sep=" ", timespec="milliseconds")
        execute(
            db, cfg,
            "UPDATE users SET username = {ph}, password_hash = {ph}, updated_at = {ph} WHERE id = {ph}",
            (args.new_username, new_hash, now, admin_id),
        )

    print("超级管理员已更新: %s → %s（数据库已生效，无需重启）" % (username, args.new_username))
    print("★ 请同步修改 config.yaml 的 admin.username / admin.password 为新值，")
    print("  否则下次重启后端会把超级管理员的账号密码对齐回配置文件里的旧值。")


def _ph(cfg):
    """按驱动返回 SQL 占位符：pymysql 用 %s，sqlite3 用 ?。"""
    return "%s" if cfg.database.driver == "mysql" else "?"


def fetchone(db, cfg, sql_template, params):
    sql = sql_template.format(ph=_ph(cfg))
    if cfg.database.driver == "mysql":
        with db.cursor() as cursor:
            cursor.execute(sql, params)
            return cursor.fetchone()
    cursor = db.execute(sql, params)
    row = cursor.fetchone()
    cursor.close()
    return row


def execute(db, cfg, sql_template, params):
    sql = sql_template.format(ph=_ph(cfg))
    if cfg.database.driver == "mysql":
        with db.cursor() as cursor:
            cursor.execute(sql, params)
    else:
        db.execute(sql, params)
    db.commit()


if __name__ == "__main__":
    main()
