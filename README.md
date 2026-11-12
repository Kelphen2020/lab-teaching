# 机房教学管理系统

高校/职校计算机机房、实训室教学管理平台。支持无盘(PXE/iSCSI)+本地盘混合终端、屏幕广播、远程控制、考试管控，兼容信创环境（麒麟/统信/UOS + x86_64/arm64/loongarch）。

## 架构

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│  Vue3 管理端 │────▶│  Go 后端 API  │◀───▶│ Rust Agent(学生机)│
│  :80/:3100  │     │  Gin :8090    │ WS  │ 心跳/指令/远控    │
└─────────────┘     └──────┬───────┘     └─────────────────┘
                           │
                    ┌──────┴───────┐
                    │ PostgreSQL 16 │  Redis 7
                    └──────────────┘
```

## 目录结构

```
lab-teaching/
├── backend/          # Go 后端 (Gin + GORM + JWT + WebSocket)
│   ├── cmd/gateway-api/   # 入口
│   ├── internal/          # api/service/repository/model/middleware/websocket/config
│   ├── pkg/               # response/auth 公共包
│   ├── migrations/        # 001_init.sql (25张表)
│   └── configs/dev.yaml   # 配置
├── frontend/         # Vue3 管理端 (Pinia + Vue Router + Vite)
│   └── src/views/         # Login/Dashboard/Rooms/Agents/Broadcast
├── agent/            # Rust 跨平台 Agent (tokio + tungstenite)
│   └── src/main.rs        # 注册/心跳/13种指令/断线重连
├── scripts/          # 运维脚本
│   ├── check_pxe_env.sh       # 无盘PXE环境检查
│   ├── deploy_agent.sh        # Agent批量部署
│   └── boot.ipxe.template     # iPXE引导脚本模板
├── deploy/           # 部署文档
└── docker-compose.yml     # 一键编排 (PG+Redis+后端+前端)
```

## 快速开始

### 方式一：Docker 一键部署
```bash
docker compose up -d --build
# 访问 http://<服务器IP>/  默认账号 admin / ChangeMe@123（可用 LAB_ADMIN_INIT_PASSWORD 覆盖，登录后立即修改）
```

### 方式二：本地开发
```bash
# 1. 数据库
sudo -u postgres createdb lab_teaching
sudo -u postgres psql lab_teaching < backend/migrations/001_init.sql

# 2. 后端 (Go 1.22+)
cd backend && go run ./cmd/gateway-api   # :8090

# 3. 前端 (Node 22+)
cd frontend && npm install && npm run dev   # :3100

# 4. Agent (Rust 1.85+)
cd agent && cargo build --release
LAB_SERVER=ws://<IP>:8090/agent/ws ./target/release/lab-agent
```

## 核心 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/auth/login | 登录获取JWT |
| GET  | /api/v1/rooms | 机房列表 |
| POST | /api/v1/rooms | 新增机房 |
| GET  | /api/v1/agents/online | Agent在线状态 |
| POST | /api/v1/ctrl/command | 下发控制指令 |
| POST | /api/v1/ctrl/broadcast | 发起屏幕广播 |
| GET  | /api/v1/stats/dashboard | 仪表盘统计 |
| WS   | /agent/ws | Agent控制通道 |

## Agent 通信协议

WebSocket 报文格式：`{ver, type, msg_id, ts, payload}`

- **type=register** — Agent注册（os/arch/mac/boot_mode）
- **type=heartbeat** — 心跳保活
- **type=cmd** — 服务端下发指令（lock/unlock/shutdown/broadcast_join/exam_enter...）
- **type=ack** — Agent执行回执

## 默认账号

- 管理员：`admin` / `ChangeMe@123`（首次启动自动创建；可用环境变量 `LAB_ADMIN_INIT_PASSWORD` 指定初始密码，登录后请立即修改）

## 许可证

[MIT](LICENSE) © 2026 北京东青互联科技有限公司（DQINFOS）

## 技术栈

- 后端：Go 1.22 + Gin + GORM + gorilla/websocket + PostgreSQL 16
- 前端：Vue 3 + Pinia + Vue Router + Vite
- Agent：Rust + tokio + tokio-tungstenite
- 部署：Docker Compose + Nginx
