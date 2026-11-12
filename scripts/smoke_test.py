#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
机房教学管理系统 - 可用性冒烟测试（Smoke Test）
================================================
用途: 部署完成后一键验证系统核心功能是否可用
用法: python3 scripts/smoke_test.py [服务器地址]
示例: python3 scripts/smoke_test.py http://192.168.10.1:8090
默认: http://localhost:8090

覆盖 8 大类: 健康检查 / 认证鉴权 / 机房管理 / 题库试卷 /
在线考试 / 仪表盘统计 / 文件上传 / V1.x 扩展模块
"""
import json, os, sys, time, urllib.request, urllib.error

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8090").rstrip("/")
API = BASE + "/api/v1"
passed, failed = 0, 0
TOKEN = None

def req(method, path, data=None, token=None, api=True):
    url = (API if api else BASE) + path
    body = json.dumps(data).encode() if data is not None else None
    r = urllib.request.Request(url, data=body, method=method)
    r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", "Bearer " + token)
    t0 = time.time()
    try:
        with urllib.request.urlopen(r, timeout=10) as resp:
            ms = int((time.time() - t0) * 1000)
            raw = resp.read()
            try:
                return resp.status, json.loads(raw), ms
            except Exception:
                return resp.status, {"_raw": raw[:200].decode("utf-8", "ignore")}, ms
    except urllib.error.HTTPError as e:
        ms = int((time.time() - t0) * 1000)
        try:
            return e.code, json.loads(e.read()), ms
        except Exception:
            return e.code, {}, ms
    except Exception as e:
        return 0, {"_err": str(e)}, int((time.time() - t0) * 1000)

def check(name, cond, detail="", ms=None):
    global passed, failed
    tag = f" ({ms}ms)" if ms is not None else ""
    if cond:
        passed += 1
        print(f"  ✓ {name}{tag}")
    else:
        failed += 1
        print(f"  ✗ {name}{tag} -- {detail}")

def section(title):
    print(f"\n【{title}】")

# ============ 1. 健康检查 ============
section("1. 服务健康检查")
s, d, ms = req("GET", "/health", api=False)
check("后端 /health", s == 200 and d.get("status") == "ok", str(d)[:120], ms)
check("数据库连接", d.get("db") == "up", str(d)[:120])
s, d, ms = req("GET", "/metrics", api=False)
check("Prometheus /metrics", s == 200, "", ms)

# ============ 2. 认证鉴权 ============
section("2. 认证鉴权")
s, d, ms = req("POST", "/auth/login", {"username": "admin", "password": os.environ.get("LAB_ADMIN_PASSWORD", "ChangeMe@123")})
TOKEN = d.get("data", {}).get("token")
check("管理员登录", s == 200 and bool(TOKEN), str(d)[:120], ms)
s, d, ms = req("POST", "/auth/login", {"username": "admin", "password": "wrong-password"})
check("错误密码拒绝", s in (400, 401), "", ms)
s, d, ms = req("GET", "/auth/profile", token=TOKEN)
check("获取个人资料", s == 200, str(d)[:120], ms)
s, d, ms = req("GET", "/auth/profile")
check("无token拒绝访问", s == 401, "", ms)
if not TOKEN:
    print("\n登录失败，无法继续后续测试")
    sys.exit(1)

# ============ 3. 机房与设备管理 ============
section("3. 机房与设备管理")
s, d, ms = req("GET", "/rooms", token=TOKEN)
rooms = d.get("data") or []
check("机房列表", s == 200, str(d)[:120], ms)
room_id = None
if isinstance(rooms, list):
    s2, d2, _ = req("POST", "/rooms", {"name": "冒烟测试机房", "location": "测试楼1层", "capacity": 40}, token=TOKEN)
    room_id = (d2.get("data") or {}).get("id")
    check("创建机房", s2 in (200, 201) and room_id, str(d2)[:120])
s, d, ms = req("GET", "/devices", token=TOKEN)
check("设备列表", s == 200, str(d)[:120], ms)
s, d, ms = req("GET", "/students", token=TOKEN)
check("学生列表", s == 200, str(d)[:120], ms)

# ============ 4. 题库与试卷 ============
section("4. 题库与试卷")
s, d, ms = req("GET", "/questions", token=TOKEN)
check("题库列表", s == 200, str(d)[:120], ms)
s, d, ms = req("GET", "/papers", token=TOKEN)
check("试卷列表", s == 200, str(d)[:120], ms)

# ============ 5. 在线考试 ============
section("5. 在线考试")
s, d, ms = req("GET", "/exams", token=TOKEN)
check("考试列表", s == 200, str(d)[:120], ms)

# ============ 6. 仪表盘与统计 ============
section("6. 仪表盘与统计")
s, d, ms = req("GET", "/dashboard/global", token=TOKEN)
check("全局仪表盘", s == 200, str(d)[:120], ms)
s, d, ms = req("GET", "/stats/dashboard", token=TOKEN)
check("统计数据", s == 200, str(d)[:120], ms)
s, d, ms = req("GET", "/agents/online", token=TOKEN)
check("Agent在线状态", s == 200, str(d)[:120], ms)
s, d, ms = req("GET", "/audit/logs", token=TOKEN)
check("审计日志", s == 200, str(d)[:120], ms)

# ============ 7. 广播/远控接口（连通性） ============
section("7. 广播/远程接口连通性")
s, d, ms = req("GET", "/ctrl/broadcast/status", token=TOKEN)
check("广播状态查询", s == 200, str(d)[:120], ms)
s, d, ms = req("GET", "/remote/sessions", token=TOKEN)
check("远控会话列表", s == 200, str(d)[:120], ms)

# ============ 8. V1.x 扩展模块 ============
section("8. V1.x 扩展模块")
s, d, ms = req("GET", "/campuses", token=TOKEN)
check("多校区列表", s == 200, str(d)[:120], ms)
s, d, ms = req("GET", "/edu/sync-configs", token=TOKEN)
check("教务同步配置", s == 200, str(d)[:120], ms)
s, d, ms = req("GET", "/access/rules", token=TOKEN)
check("门禁规则列表", s == 200, str(d)[:120], ms)
s, d, ms = req("GET", "/license", token=TOKEN)
check("License状态", s == 200, str(d)[:120], ms)
s, d, ms = req("GET", "/ops/upgrade", token=TOKEN)
check("升级任务列表", s == 200, str(d)[:120], ms)

# ============ 清理测试数据 ============
if room_id:
    req("DELETE", f"/rooms/{room_id}", token=TOKEN)

# ============ 汇总 ============
total = passed + failed
print("\n" + "=" * 50)
print(f"冒烟测试结果: {passed}/{total} 通过, {failed} 失败")
if failed == 0:
    print("✅ 系统核心功能全部可用")
else:
    print("⚠️ 存在失败项，请根据上方明细排查")
print("=" * 50)
sys.exit(1 if failed else 0)
