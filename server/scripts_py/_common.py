# -*- coding: utf-8 -*-
"""scripts_py 公共模块：配置加载、数据库连接、七牛 REST 客户端、后端 HTTP 请求与限速。

与 Go 侧的对应关系（行为保持一致）：
  - load_config()       ← config.Load()（config/config.go：默认值 → YAML → 环境变量 → 归一化 → 必填校验）
  - open_database()     ← database.Open（database/database.go：SQLite 自动建目录）
  - QiniuClient         ← qbox.Mac + BucketManager（ListFiles / ChangeMeta / Batch / Buckets / ListBucketDomains）
  - do_json / do_upload / login / Throttle ← scripts/seed 与 scripts/seedmedia 中的同名逻辑

依赖：pyyaml（配置文件解析，pip install pyyaml）、pymysql（仅 driver=mysql 时导入）。
七牛相关脚本只用标准库（hmac/hashlib/http.client/urllib），不依赖任何七牛 SDK。

七牛签名（与 go-sdk v7 的 Qiniu V2 签名逐字节一致）：
  data  = "METHOD /path?query\\nHost: host\\nContent-Type: ct\\n\\n" + body（Content-Type 为 form/json 时）
  token = base64url(hmac_sha1(secret_key, data))
  头部  = Authorization: Qiniu <access_key>:<token>
区域与主机（Go 侧 NewBucketManager 传入 storage.Config.Region，走 SDK 静态区域表，
不发起 uc v4/query；域名按 use_https 配置拼 http/https）：
  /list      → rsf.<zone>（list 走 rsf 入口，见 bucket_list.go RsfReqHost）
  /chgm /batch → rs.<zone>
  /buckets /v3/domains → uc.qiniuapi.com

运行约定：全部脚本都从 server 目录下运行（与 Go 版一致，配置文件按当前目录解析）。
"""
from __future__ import annotations

import base64
import copy
import hashlib
import hmac
import json
import os
import re
import sys
import threading
import time
from http.client import HTTPConnection, HTTPSConnection
from types import SimpleNamespace
from urllib.error import HTTPError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

FORM_CONTENT_TYPE = "application/x-www-form-urlencoded"
JSON_CONTENT_TYPE = "application/json"

DEFAULTS = {
    "server": {
        "port": "8080",
        "jwt_secret": "",
        "upload_dir": "./storage/media",
        "log_level": "info",
        "log_dir": "./logs",
        "web_dist_dir": "../web/dist",
        "reveal_free_per_day": 5,
    },
    "database": {
        "driver": "sqlite",
        "sqlite": {"path": "./storage/rental.db"},
        "mysql": {
            "host": "127.0.0.1",
            "port": "3306",
            "user": "root",
            "password": "",
            "name": "rental",
        },
    },
    "storage": {
        "media_max_age": 604800,
        "qiniu": {
            "access_key": "",
            "secret_key": "",
            "bucket": "",
            "domain": "",
            "use_https": False,
            "zone": "z2",
            "proxy_mode": True,
            "pfop_enable": False,
            "pfop_callback_secret": "",
            "pfop_callback_url": "",
        },
    },
}

_STR_ENV = {
    "DB_DRIVER": ("database", "driver"),
    "DB_PATH": ("database", "sqlite", "path"),
    "DB_HOST": ("database", "mysql", "host"),
    "DB_PORT": ("database", "mysql", "port"),
    "DB_USER": ("database", "mysql", "user"),
    "DB_PASSWORD": ("database", "mysql", "password"),
    "DB_NAME": ("database", "mysql", "name"),
    "JWT_SECRET": ("server", "jwt_secret"),
    "SERVER_PORT": ("server", "port"),
    "UPLOAD_DIR": ("server", "upload_dir"),
    "LOG_LEVEL": ("server", "log_level"),
    "LOG_DIR": ("server", "log_dir"),
    "WEB_DIST_DIR": ("server", "web_dist_dir"),
    "QINIU_ACCESS_KEY": ("storage", "qiniu", "access_key"),
    "QINIU_SECRET_KEY": ("storage", "qiniu", "secret_key"),
    "QINIU_BUCKET": ("storage", "qiniu", "bucket"),
    "QINIU_DOMAIN": ("storage", "qiniu", "domain"),
    "QINIU_ZONE": ("storage", "qiniu", "zone"),
}

_BOOL_ENV = {
    "QINIU_USE_HTTPS": ("storage", "qiniu", "use_https"),
    "QINIU_PROXY_MODE": ("storage", "qiniu", "proxy_mode"),
    "QINIU_PFOP_ENABLE": ("storage", "qiniu", "pfop_enable"),
}


def die(msg):
    sys.stderr.write("panic: %s\n" % msg)
    sys.exit(2)


RS_HOSTS = {
    "z0": "rs.qbox.me",
    "z1": "rs-z1.qbox.me",
    "z2": "rs-z2.qbox.me",
    "na0": "rs-na0.qbox.me",
    "as0": "rs-as0.qbox.me",
}

RSF_HOSTS = {
    "z0": "rsf.qbox.me",
    "z1": "rsf-z1.qbox.me",
    "z2": "rsf-z2.qbox.me",
    "na0": "rsf-na0.qbox.me",
    "as0": "rsf-as0.qbox.me",
}

UC_HOST = "uc.qiniuapi.com"


def _deep_merge(base, override):
    for key, value in override.items():
        if isinstance(value, dict) and isinstance(base.get(key), dict):
            _deep_merge(base[key], value)
        elif value is not None:
            base[key] = value


def _set_path(cfg, path, value):
    node = cfg
    for part in path[:-1]:
        node = node[part]
    node[path[-1]] = value


def _get_path(cfg, path):
    node = cfg
    for part in path:
        node = node[part]
    return node


def _to_ns(obj):
    if isinstance(obj, dict):
        return SimpleNamespace(**{k: _to_ns(v) for k, v in obj.items()})
    if isinstance(obj, list):
        return [_to_ns(v) for v in obj]
    return obj


def load_config():
    cfg = copy.deepcopy(DEFAULTS)

    path = os.environ.get("CONFIG_PATH") or "config.yaml"
    try:
        with open(path, "rb") as fh:
            data = fh.read()
    except OSError:
        data = None
    if data is not None:
        try:
            import yaml
        except ImportError:
            die("缺少 pyyaml 依赖，请先执行: pip install pyyaml")
        try:
            parsed = yaml.safe_load(data)
        except Exception as exc:  # yaml.YAMLError
            die("配置解析失败: %s" % exc)
        if isinstance(parsed, dict):
            _deep_merge(cfg, parsed)

    for env_name, env_path in _STR_ENV.items():
        value = os.environ.get(env_name)
        if value:
            _set_path(cfg, env_path, value)
    for env_name, env_path in _BOOL_ENV.items():
        value = os.environ.get(env_name)
        if value:
            _set_path(cfg, env_path, value == "true")
    pfop_secret = os.environ.get("QINIU_PFOP_CALLBACK_SECRET")
    if pfop_secret:
        _set_path(cfg, ("storage", "qiniu", "pfop_callback_secret"), pfop_secret)
    pfop_url = os.environ.get("QINIU_PFOP_CALLBACK_URL")
    if pfop_url:
        _set_path(cfg, ("storage", "qiniu", "pfop_callback_url"), pfop_url)
    media_max_age = os.environ.get("MEDIA_MAX_AGE")
    if media_max_age is not None and re.fullmatch(r"[+-]?\d+", media_max_age.strip() or "x"):
        _set_path(cfg, ("storage", "media_max_age"), int(media_max_age))

    driver = str(cfg["database"]["driver"] or "").strip().lower()
    if not driver:
        driver = "sqlite"
    if driver not in ("mysql", "sqlite"):
        die('不支持的 database.driver: "%s"（可选 mysql / sqlite）' % driver)
    cfg["database"]["driver"] = driver
    if not cfg["database"]["sqlite"]["path"]:
        cfg["database"]["sqlite"]["path"] = "./storage/rental.db"
    if cfg["server"]["reveal_free_per_day"] < 0:
        cfg["server"]["reveal_free_per_day"] = 5

    if driver == "mysql" and not cfg["database"]["mysql"]["password"]:
        die("database password is required: set database.mysql.password in config.yaml or DB_PASSWORD env")
    if not cfg["server"]["jwt_secret"]:
        die("jwt secret is required: set server.jwt_secret in config.yaml or JWT_SECRET env")

    return _to_ns(cfg)


def media_cache_control(cfg):
    age = cfg.storage.media_max_age
    if age <= 0:
        age = 604800
    return "public, max-age=%d, immutable" % age


def open_database(cfg):
    if cfg.database.driver == "mysql":
        import pymysql

        mysql = cfg.database.mysql
        return pymysql.connect(
            host=mysql.host,
            port=int(mysql.port),
            user=mysql.user,
            password=mysql.password,
            database=mysql.name,
            charset="utf8mb4",
            autocommit=True,
        )
    import sqlite3

    db_path = cfg.database.sqlite.path
    parent = os.path.dirname(db_path)
    if parent:
        os.makedirs(parent, exist_ok=True)
    return sqlite3.connect(db_path)


class QiniuError(Exception):
    def __init__(self, status, message):
        super().__init__("HTTP %d: %s" % (status, message))
        self.status = status
        self.message = message


def _b64(value):
    if isinstance(value, str):
        value = value.encode("utf-8")
    return base64.urlsafe_b64encode(value).decode("ascii")


class QiniuClient:
    def __init__(self, access_key, secret_key, zone="z2", use_https=False):
        self.access_key = access_key
        self.secret_key = secret_key.encode("utf-8")
        self.zone = zone if zone in RS_HOSTS else "z2"
        self.use_https = use_https

    @property
    def _scheme(self):
        return "https" if self.use_https else "http"

    def _rs_host(self):
        return RS_HOSTS[self.zone]

    def _rsf_host(self):
        return RSF_HOSTS[self.zone]

    def _uc_host(self):
        return UC_HOST

    def _call(self, method, host, target, body=None):
        content_type = FORM_CONTENT_TYPE
        sign_data = "%s %s\nHost: %s\nContent-Type: %s\n\n" % (method, target, host, content_type)
        data = sign_data.encode("utf-8")
        if body is not None:
            data += body
        digest = hmac.new(self.secret_key, data, hashlib.sha1).digest()
        token = base64.urlsafe_b64encode(digest).decode("ascii")
        connection_cls = HTTPSConnection if self._scheme == "https" else HTTPConnection
        connection = connection_cls(host, timeout=60)
        try:
            connection.request(
                method,
                target,
                body=body,
                headers={
                    "Content-Type": content_type,
                    "Authorization": "Qiniu %s:%s" % (self.access_key, token),
                },
            )
            response = connection.getresponse()
            payload = response.read()
            status = response.status
        finally:
            connection.close()
        if status >= 400:
            try:
                message = json.loads(payload.decode("utf-8")).get("error") or payload.decode("utf-8", "replace")
            except Exception:
                message = payload.decode("utf-8", "replace")
            raise QiniuError(status, message)
        return payload

    def list_files(self, bucket, prefix="", delimiter="", marker="", limit=0):
        pairs = [("bucket", bucket)]
        if prefix:
            pairs.append(("prefix", prefix))
        if delimiter:
            pairs.append(("delimiter", delimiter))
        if marker:
            pairs.append(("marker", marker))
        if limit > 0:
            pairs.append(("limit", str(limit)))
        pairs.append(("needparts", "false"))
        pairs.sort(key=lambda item: item[0])
        payload = self._call("POST", self._rsf_host(), "/list?" + urlencode(pairs))
        data = json.loads(payload.decode("utf-8"))
        return data.get("items") or [], data.get("marker") or ""

    def change_meta(self, bucket, key, metas):
        segments = ["/chgm/" + _b64("%s:%s" % (bucket, key))]
        for meta_key, meta_value in metas.items():
            if not meta_key.startswith("x-qn-meta-"):
                meta_key = "x-qn-meta-" + meta_key
            segments.append("/" + meta_key)
            segments.append("/" + _b64(meta_value))
        self._call("POST", self._rs_host(), "".join(segments))

    def batch(self, operations):
        body = urlencode([("op", op) for op in operations]).encode("utf-8")
        payload = self._call("POST", self._rs_host(), "/batch", body=body)
        return json.loads(payload.decode("utf-8"))

    def buckets(self, shared="rw"):
        payload = self._call("GET", self._uc_host(), "/buckets?" + urlencode([("shared", shared)]))
        return json.loads(payload.decode("utf-8"))

    def list_domains(self, bucket):
        payload = self._call("GET", self._uc_host(), "/v3/domains?" + urlencode([("tbl", bucket)]))
        data = json.loads(payload.decode("utf-8"))
        return [item.get("domain", "") for item in data if isinstance(item, dict)]


def uri_copy(src_bucket, src_key, dst_bucket, dst_key, force):
    return "/copy/%s/%s/force/%s" % (
        _b64("%s:%s" % (src_bucket, src_key)),
        _b64("%s:%s" % (dst_bucket, dst_key)),
        "true" if force else "false",
    )


BASE_URL = os.environ.get("SEED_BASE_URL") or "http://127.0.0.1:8081"


def env_int(name, default):
    value = os.environ.get(name)
    if value:
        match = re.match(r"[+-]?\d+", value.strip())
        if match:
            return int(match.group(0))
    return default


class Throttle:
    def __init__(self, per_minute):
        self.min_interval = 60.0 / per_minute if per_minute > 0 else 0.0
        self._lock = threading.Lock()
        self._next = 0.0

    def wait(self):
        if self.min_interval <= 0:
            return
        with self._lock:
            now = time.monotonic()
            if self._next > now:
                time.sleep(self._next - now)
                now = time.monotonic()
            self._next = now + self.min_interval


REQ_LIMITER = Throttle(env_int("SEED_RATE_PER_MIN", 200))


class ApiError(Exception):
    def __init__(self, status, code, message):
        super().__init__("HTTP %d (code %d): %s" % (status, code, message))
        self.status = status
        self.code = code
        self.message = message


def _gofloat(obj):
    if isinstance(obj, dict):
        return {key: _gofloat(value) for key, value in obj.items()}
    if isinstance(obj, list):
        return [_gofloat(value) for value in obj]
    if isinstance(obj, float) and obj.is_integer() and abs(obj) < 2**53:
        return int(obj)
    return obj


def _do_once(method, path, token, content_type, body_bytes):
    REQ_LIMITER.wait()
    headers = {"Content-Type": content_type}
    if token:
        headers["Authorization"] = "Bearer " + token
    request = Request(BASE_URL + path, data=body_bytes, method=method, headers=headers)
    try:
        with urlopen(request) as response:
            status = response.status
            payload = response.read()
    except HTTPError as exc:
        status = exc.code
        payload = exc.read()
    try:
        parsed = json.loads(payload.decode("utf-8"))
    except Exception as exc:
        raise RuntimeError("解析响应失败: %s" % exc) from exc
    if status >= 400 or parsed.get("code", 0) != 0:
        raise ApiError(status, parsed.get("code", 0), parsed.get("message", ""))
    data = parsed.get("data")
    if data is None or data == "" or data == []:
        return None
    return data


def do_req(method, path, token, content_type, body_bytes):
    last_error = None
    for attempt in range(10):
        if attempt > 0:
            time.sleep(attempt * 0.5)
        try:
            return _do_once(method, path, token, content_type, body_bytes)
        except ApiError as exc:
            last_error = exc
            if exc.status == 429:
                time.sleep(65)
                continue
            if exc.status < 500:
                raise exc
            continue
        except Exception as exc:
            last_error = exc
            continue
    raise last_error


def do_json(method, path, token, body=None):
    body_bytes = None
    if body is not None:
        body_bytes = json.dumps(_gofloat(body), ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    return do_req(method, path, token, JSON_CONTENT_TYPE, body_bytes)


def do_upload(path, token, field, filename, data, extra=None):
    boundary = "----scriptsPyBoundary%s" % hashlib.md5(os.urandom(16)).hexdigest()
    chunks = []
    head = "--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n" % (
        boundary,
        field,
        filename,
    )
    chunks.append(head.encode("utf-8"))
    chunks.append(b"Content-Type: application/octet-stream\r\n\r\n")
    chunks.append(data)
    chunks.append(b"\r\n")
    for key, value in (extra or {}).items():
        chunks.append(("--%s\r\nContent-Disposition: form-data; name=\"%s\"\r\n\r\n%s\r\n" % (boundary, key, value)).encode("utf-8"))
    chunks.append(("--%s--\r\n" % boundary).encode("utf-8"))
    content_type = "multipart/form-data; boundary=" + boundary
    return do_req("POST", path, token, content_type, b"".join(chunks))


def login(username, password):
    data = do_json("POST", "/api/auth/login", "", {"username": username, "password": password})
    return data["token"]
