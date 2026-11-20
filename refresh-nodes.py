#!/usr/bin/env python3
"""节点池自动刷新：
1) 拉取订阅（Clash YAML）
2) 生成 sing-box 配置（一个节点 = 一个本地端口 = 一个出口 IP）
3) 校验并重启 singbox-pool
4) 让 Go 工具重新测活并热更新节点池（批量不中断）
不动 magpie 等其它服务。
"""
import datetime
import json
import os
import shutil
import subprocess
import sys
import time
import urllib.request
import urllib.parse

import yaml

PW = "pass" + "word"
SUB_URL_FILE = "/opt/minimax/sub-url.txt"
BASE_PORT = int(os.environ.get("MINIMAX_BASE_PORT", "18001"))
YAML_PATH = "/opt/minimax/singlebox-clash.yaml"
CFG_PATH = "/opt/minimax/singbox-config.json"
MANIFEST_PATH = "/opt/minimax/singbox-nodes.json"
TOOL_URL = "http://127.0.0.1:8090"
UNIT_FILE = "/etc/systemd/system/minimax-web.service"
# wireguard 在 sing-box 1.14 已迁到 endpoint 类型且需要 TUN/系统接口，
# 订阅里仅 5 个，暂不接入以免拖累整体配置。
SUPPORTED = ("ss", "vmess", "vless", "trojan", "hysteria2", "anytls",
             "http", "https")

# 额外的 VLESS 链接订阅（多条订阅合并进同一个 sing-box 节点池）
VLESS_SUB_URL = os.environ.get(
    "MINIMAX_VLESS_SUB_URL",
    "https://raw.githubusercontent.com/igareck/vpn-configs-for-russia/"
    "refs/heads/main/BLACK_VLESS_RUS_mobile.txt")
VLESS_PATH = "/opt/minimax/vless-sub.txt"


def log(msg):
    print("[%s] %s" % (datetime.datetime.now().strftime("%H:%M:%S"), msg), flush=True)


def subscription_url():
    env = os.environ.get("MINIMAX_SUB_URL")
    if env:
        return env.strip()
    try:
        with open(SUB_URL_FILE, encoding="utf-8") as fh:
            return fh.read().strip()
    except Exception:
        raise SystemExit("未找到订阅地址（%s）" % SUB_URL_FILE)


def tool_token():
    try:
        with open(UNIT_FILE, encoding="utf-8") as fh:
            for line in fh:
                if "-token" in line:
                    parts = line.strip().split()
                    if "-token" in parts:
                        return parts[parts.index("-token") + 1]
                    return parts[-1]
    except Exception:
        pass
    return ""


def api(name, args=None, timeout=900):
    body = json.dumps({"name": name, "args": args or []}).encode()
    req = urllib.request.Request(
        TOOL_URL + "/api/call",
        data=body,
        headers={"Content-Type": "application/json", "X-MiniMax-Token": tool_token()},
    )
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode())


def fetch_subscription():
    req = urllib.request.Request(subscription_url(), headers={"User-Agent": "Clash/v1.18.0"})
    with urllib.request.urlopen(req, timeout=60) as resp:
        data = resp.read()
    with open(YAML_PATH, "wb") as fh:
        fh.write(data)
    log("1) 订阅已下载 %d 字节" % len(data))
    return yaml.safe_load(data.decode("utf-8", "replace"))


def tls_block(p):
    if not p.get("tls"):
        return None
    t = {"enabled": True, "insecure": bool(p.get("skip-cert-verify"))}
    sni = p.get("sni") or p.get("servername")
    if sni:
        t["server_name"] = sni
    alpn = p.get("alpn")
    if alpn:
        t["alpn"] = alpn if isinstance(alpn, list) else [alpn]
    ro = p.get("reality-opts") or {}
    if ro.get("public-key"):
        t["reality"] = {
            "enabled": True,
            "public_key": ro.get("public-key"),
            "short_id": ro.get("short-id") or "",
        }
    fp = p.get("client-fingerprint")
    if fp and fp != "unsafe":
        t["utls"] = {"enabled": True, "fingerprint": fp}
    return t


def transport_block(p):
    net = p.get("network") or "tcp"
    if net == "ws":
        wo = p.get("ws-opts") or {}
        hdrs = wo.get("headers") or {}
        tr = {"type": "ws"}
        if wo.get("path"):
            tr["path"] = wo["path"]
        host = hdrs.get("Host") or hdrs.get("host")
        if host:
            tr["headers"] = {"Host": host}
        return tr
    if net == "grpc":
        go = p.get("grpc-opts") or {}
        return {"type": "grpc", "service_name": go.get("grpc-service-name") or ""}
    if net == "h2":
        ho = p.get("h2-opts") or {}
        tr = {"type": "http"}
        host = ho.get("host")
        if host:
            tr["host"] = host if isinstance(host, list) else [host]
        if ho.get("path"):
            tr["path"] = ho["path"]
        return tr
    return None


def build_outbound(p, tag):
    t = p.get("type")
    if t in ("http", "https"):
        # singlebox 的转换器节点：type=http + tls=true 走 TLS 到转换器再转发
        ob = {
            "type": "http",
            "tag": tag,
            "server": p["server"],
            "server_port": int(p["port"]),
        }
        if t == "https" or p.get("tls"):
            ob["tls"] = {"enabled": True, "insecure": bool(p.get("skip-cert-verify"))}
            sni = p.get("sni")
            if sni:
                ob["tls"]["server_name"] = sni
        if p.get("username"):
            ob["username"] = p["username"]
            ob[PW] = p.get(PW) or ""
        return ob
    ob = {
        "type": {"ss": "shadowsocks", "ssr": "shadowsocksr", "hy2": "hysteria2"}.get(t, t),
        "tag": tag,
        "server": p["server"],
        "server_port": int(p["port"]),
    }
    if t == "ss":
        ob["method"] = p.get("cipher") or "aes-256-gcm"
        ob[PW] = p.get(PW) or ""
    elif t == "vmess":
        ob["uuid"] = p.get("uuid") or ""
        ob["security"] = p.get("cipher") or "auto"
        ob["alter_id"] = int(p.get("alterId") or 0)
        tb = tls_block(p)
        if tb:
            ob["tls"] = tb
        tr = transport_block(p)
        if tr:
            ob["transport"] = tr
    elif t == "vless":
        ob["uuid"] = p.get("uuid") or ""
        if p.get("flow"):
            ob["flow"] = p["flow"]
        tb = tls_block(p)
        if tb:
            ob["tls"] = tb
        tr = transport_block(p)
        if tr:
            ob["transport"] = tr
    elif t == "trojan":
        ob[PW] = p.get(PW) or ""
        tb = tls_block(p) or {"enabled": True, "insecure": bool(p.get("skip-cert-verify"))}
        ob["tls"] = tb
        tr = transport_block(p)
        if tr:
            ob["transport"] = tr
    elif t == "hysteria2":
        ob[PW] = p.get(PW) or p.get("auth") or ""
        if p.get("obfs"):
            ob["obfs"] = {"type": p["obfs"], PW: p.get("obfs-" + PW) or ""}
        ob["tls"] = tls_block(p) or {"enabled": True, "insecure": bool(p.get("skip-cert-verify"))}
        if p.get("up"):
            ob["up_mbps"] = int(p["up"])
        if p.get("down"):
            ob["down_mbps"] = int(p["down"])
    elif t == "anytls":
        ob[PW] = p.get(PW) or ""
        tb = tls_block(p)
        # anytls 必须开启 TLS
        ob["tls"] = tb or {"enabled": True, "insecure": True}
    else:
        return None
    return ob


def build_config(doc, vless=None):
    inbounds, outbounds, rules, manifest = [], [], [], []
    idx = 0
    proxies = list(doc.get("proxies") or [])
    if vless is None:
        vless = load_vless_proxies()
    proxies.extend(vless)
    for p in proxies:
        if p.get("type") not in SUPPORTED or not p.get("server") or not p.get("port"):
            continue
        port = BASE_PORT + idx
        ob = build_outbound(p, "out-%d" % idx)
        if ob is None:
            continue
        inbounds.append({
            "type": "mixed",
            "tag": "in-%d" % idx,
            "listen": "127.0.0.1",
            "listen_port": port,
        })
        outbounds.append(ob)
        rules.append({"action": "route", "inbound": ["in-%d" % idx], "outbound": "out-%d" % idx})
        manifest.append({
            "port": port,
            "name": str(p.get("name") or p["server"]),
            "type": p.get("type"),
            "server": p.get("server"),
        })
        idx += 1
    if not outbounds:
        raise SystemExit("订阅里没有可用节点")
    cfg = {
        "log": {"level": "warn", "timestamp": True},
        "inbounds": inbounds,
        "outbounds": outbounds,
        "route": {"rules": rules, "final": outbounds[0]["tag"]},
    }
    return cfg, manifest


def load_vless_proxies():
    """读取 VLESS 链接订阅，转成 Clash 风格的 proxy 字典（拉取失败不阻断刷新）。"""
    try:
        req = urllib.request.Request(
            VLESS_SUB_URL, headers={"User-Agent": "Clash/v1.18.0"})
        with urllib.request.urlopen(req, timeout=60) as resp:
            data = resp.read()
        with open(VLESS_PATH, "wb") as fh:
            fh.write(data)
        proxies = parse_vless_links(data.decode("utf-8", "replace"))
        log("0) VLESS 订阅拉取成功：%d 条" % len(proxies))
        return proxies
    except Exception as exc:
        log("0) VLESS 订阅拉取失败（忽略）：%s" % exc)
        return []


def parse_vless_links(text):
    """把 vless://uuid@host:port?type=ws&path=...&security=tls#name 解析成 proxy 字典。"""
    out = []
    seen = set()
    for raw in text.splitlines():
        line = raw.strip()
        if not line.startswith("vless://"):
            continue
        try:
            rest = line[len("vless://"):]
            frag = ""
            if "#" in rest:
                rest, frag = rest.split("#", 1)
            query = ""
            if "?" in rest:
                rest, query = rest.split("?", 1)
            if "@" not in rest:
                continue
            userinfo, hostport = rest.rsplit("@", 1)
            host, _, port = hostport.partition(":")
            if not host or not port.isdigit():
                continue
            params = urllib.parse.parse_qs(query)

            def one(key, default=""):
                vals = params.get(key) or []
                return vals[0] if vals else default

            net = (one("type", "tcp") or "tcp").lower()
            if net not in ("tcp", "ws", "grpc", "h2", "http"):
                net = "tcp"
            proxy = {
                "type": "vless",
                "name": urllib.parse.unquote(frag) or ("%s:%s" % (host, port)),
                "server": host,
                "port": int(port),
                "uuid": userinfo,
                "network": net,
            }
            security = (one("security", "none") or "none").lower()
            if security in ("tls", "reality"):
                proxy["tls"] = True
                proxy["skip-cert-verify"] = True
                sni = one("sni") or one("host")
                if sni:
                    proxy["sni"] = sni
            fp = one("fp")
            if fp:
                proxy["client-fingerprint"] = fp
            alpn = one("alpn")
            if alpn:
                proxy["alpn"] = [a for a in alpn.split(",") if a]
            if net == "ws":
                path = one("path", "/")
                # 订阅里路径常被二次编码，解两次还原成真实路径
                for _ in range(2):
                    path = urllib.parse.unquote(path)
                host_hdr = one("host")
                opts = {"path": path or "/"}
                if host_hdr:
                    opts["headers"] = {"Host": host_hdr}
                proxy["ws-opts"] = opts
            elif net == "grpc":
                proxy["grpc-opts"] = {
                    "grpc-service-name": one("serviceName") or one("servicename")}
            key = (proxy["server"], proxy["port"], proxy["uuid"])
            if key in seen:
                continue
            seen.add(key)
            out.append(proxy)
        except Exception:
            continue
    return out


def apply_config(cfg, manifest):
    tmp = CFG_PATH + ".new"
    with open(tmp, "w", encoding="utf-8") as fh:
        json.dump(cfg, fh, ensure_ascii=False, indent=1)
    r = subprocess.run(["sing-box", "check", "-c", tmp], capture_output=True, text=True)
    if r.returncode != 0:
        log("2) 配置校验失败，保留旧配置: %s" % (r.stderr or r.stdout)[:200])
        os.remove(tmp)
        return False
    os.replace(tmp, CFG_PATH)
    with open(MANIFEST_PATH, "w", encoding="utf-8") as fh:
        json.dump(manifest, fh, ensure_ascii=False, indent=1)
    log("2) sing-box 配置已生成：%d 个节点，端口 %d-%d" % (
        len(manifest), manifest[0]["port"], manifest[-1]["port"]))
    subprocess.run(["systemctl", "restart", "singbox-pool"], check=False)
    log("3) singbox-pool 已重启")
    wait_ports(manifest[0]["port"], manifest[-1]["port"])
    return True


def wait_ports(lo, hi, timeout=30):
    """等 sing-box 端口全部监听完成，避免刚重启就测活导致误判全灭。"""
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            out = subprocess.run(["ss", "-ltn"], capture_output=True, text=True, timeout=10).stdout
        except Exception:
            out = ""
        listening = sum(1 for line in out.split("\n") if ":%d " % lo in line or ":%d " % hi in line)
        if listening >= 2:
            time.sleep(2)
            return True
        time.sleep(2)
    log("3) 等待端口监听超时，继续测活")
    return False


def refresh_tool(manifest):
    lo = manifest[0]["port"]
    hi = manifest[-1]["port"]
    cfg = api("GetConfig").get("data") or {}
    cfg["proxyLocalPorts"] = "%d-%d" % (lo, hi)
    api("SaveConfig", [cfg])
    api("ImportProxySubscription", [{}])
    pool_path = (cfg.get("proxyPoolFile") or "/opt/minimax/artifacts/proxy-pool.json")
    backup = pool_path + ".bak"
    shutil.copy2(pool_path, backup)
    old_alive = 0
    try:
        with open(backup, encoding="utf-8") as fh:
            old_alive = sum(1 for n in (json.load(fh).get("nodes") or []) if n.get("alive"))
    except Exception:
        pass
    for attempt in (1, 2):
        res = api("CheckProxyPool", [{}])
        data = res.get("data") or {}
        alive = data.get("alive") or 0
        if alive > 0 or attempt == 2:
            log("4) 节点池测活完成：存活 %s / %s" % (alive, data.get("total")))
            if alive == 0 and old_alive > 0:
                shutil.copy2(backup, pool_path)
                log("4) 本轮全部失败，已回滚到旧节点池（存活 %d）" % old_alive)
                return {"alive": old_alive, "total": data.get("total")}
            return data
        log("4) 第 %d 轮测活为 0，15s 后重试" % attempt)
        time.sleep(15)
    return {}


def main():
    doc = fetch_subscription()
    vless = load_vless_proxies()
    cfg, manifest = build_config(doc, vless)
    if not apply_config(cfg, manifest):
        # 含 VLESS 节点时若整份配置校验不过，退回仅 Clash 节点
        cfg, manifest = build_config(doc, [])
        if not apply_config(cfg, manifest):
            return 1
    try:
        refresh_tool(manifest)
    except Exception as exc:
        log("4) 节点池刷新失败（不影响批量继续跑）: %s" % exc)
    return 0


if __name__ == "__main__":
    sys.exit(main())

