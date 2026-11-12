#!/usr/bin/env bash
# ============================================================
# 机房教学管理系统 - 服务器一键安装脚本（二进制方式）
# 支持: x86_64 / aarch64 / loongarch64
# 用法: sudo bash install_server.sh
# 说明: 在解压后的 lab-teaching 目录内执行
# ============================================================
set -e

INSTALL_DIR=/opt/lab-teaching
RUN_USER=lab

echo "=== [1/6] 检测架构 ==="
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)       BIN=dist/gateway-api-linux-amd64 ;;
    aarch64)      BIN=dist/gateway-api-linux-arm64 ;;
    loongarch64)  BIN=dist/gateway-api-linux-loong64 ;;
    *) echo "不支持的架构: $ARCH"; exit 1 ;;
esac
[ -f "$BIN" ] || { echo "未找到 $BIN，请先构建或使用全量发布包"; exit 1; }
echo "选用二进制: $BIN ($ARCH)"

echo "=== [2/6] 创建运行用户 ==="
id -u $RUN_USER >/dev/null 2>&1 || useradd -r -s /sbin/nologin $RUN_USER

echo "=== [3/6] 安装文件 ==="
mkdir -p $INSTALL_DIR/{configs,logs,backups,data}
cp "$BIN" $INSTALL_DIR/gateway-api
chmod +x $INSTALL_DIR/gateway-api
[ -f deploy/server/production.yaml ] && cp -n deploy/server/production.yaml $INSTALL_DIR/configs/production.yaml
chown -R $RUN_USER:$RUN_USER $INSTALL_DIR

echo "=== [4/6] 安装 systemd 服务 ==="
cp deploy/server/gateway-api.service /etc/systemd/system/gateway-api.service
systemctl daemon-reload

echo "=== [5/6] 配置提示 ==="
echo "请修改数据库配置后启动:"
echo "  vi $INSTALL_DIR/configs/production.yaml"
echo "  (修改 database.password 与 jwt.secret)"
echo "启动命令:"
echo "  systemctl enable --now gateway-api"
echo "查看日志:"
echo "  journalctl -u gateway-api -f"

echo "=== [6/6] 完成 ==="
echo "安装目录: $INSTALL_DIR"
echo "默认账号: admin / ChangeMe@123 (可用 LAB_ADMIN_INIT_PASSWORD 覆盖，首次登录后请立即修改)"
