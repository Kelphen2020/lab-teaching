#!/usr/bin/env bash
# ============================================================
# 机房教学管理系统 - Agent 批量部署脚本（本地盘终端）
# 用途: 向已装系统的学生机批量推送并安装 Agent
# 用法: sudo bash scripts/deploy_agent.sh <IP段> <服务器IP>
# 示例: sudo bash scripts/deploy_agent.sh 192.168.10 192.168.10.1
# ============================================================
set -u

SUBNET="${1:-192.168.10}"
SERVER="${2:?用法: sudo bash scripts/deploy_agent.sh <网段> <服务器IP>，例如 192.168.10 192.168.10.1}"
AGENT_BIN="lab-agent"
DIST_DIR="dist"
SSH_USER="root"
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=3"

echo "======================================================"
echo " Agent 批量部署（支持 统信UOS/银河麒麟 + x86/ARM/龙芯）"
echo " 目标网段: ${SUBNET}.1-254   服务器: ${SERVER}"
echo "======================================================"

# 检查本地是否有 agent 二进制（优先多架构 dist，回退默认 release）
if [ ! -f "agent/target/release/${AGENT_BIN}" ]; then
    echo "[!] 未找到 agent/target/release/${AGENT_BIN}，请先 cargo build --release"
    echo "    或运行 scripts/build_multiarch.sh 构建多架构版本"
    exit 1
fi

# 根据目标机架构选择对应二进制
pick_binary() {
    local arch="$1"
    case "$arch" in
        x86_64)  echo "${DIST_DIR}/lab-agent-linux-amd64" ;;
        aarch64) echo "${DIST_DIR}/lab-agent-linux-arm64" ;;
        loongarch64) echo "${DIST_DIR}/lab-agent-linux-loong64" ;;
        *) echo "" ;;
    esac
}

deploy_one() {
    local ip=$1
    # 1. 探测主机存活
    ping -c1 -W1 "$ip" >/dev/null 2>&1 || return
    echo "[+] $ip 在线，开始部署..."

    # 2. 探测目标机架构与操作系统
    local remote_arch remote_os bin
    remote_arch=$(ssh $SSH_OPTS "${SSH_USER}@${ip}" "uname -m" 2>/dev/null) || {
        echo "    [-] $ip 无法 SSH 登录"; return; }
    remote_os=$(ssh $SSH_OPTS "${SSH_USER}@${ip}" ". /etc/os-release 2>/dev/null && echo \$NAME" 2>/dev/null || echo "unknown")

    # 3. 按架构选择二进制（多架构 dist 优先，否则回退本机构建）
    bin=$(pick_binary "$remote_arch")
    if [ -n "$bin" ] && [ -f "$bin" ]; then
        echo "    目标: ${remote_os} / ${remote_arch} -> $(basename $bin)"
    else
        bin="agent/target/release/${AGENT_BIN}"
        echo "    目标: ${remote_os} / ${remote_arch} -> 默认二进制"
    fi

    # 4. 推送二进制
    scp $SSH_OPTS "$bin" "${SSH_USER}@${ip}:/usr/local/bin/${AGENT_BIN}" 2>/dev/null || {
        echo "    [-] $ip 推送失败（检查 SSH 免密）"; return; }

    # 5. 写入 systemd 服务
    ssh $SSH_OPTS "${SSH_USER}@${ip}" "cat > /etc/systemd/system/lab-agent.service" <<EOF
[Unit]
Description=Lab Teaching Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment=LAB_SERVER=ws://${SERVER}:8090/agent/ws
Environment=LAB_BOOT_MODE=本地
ExecStart=/usr/local/bin/${AGENT_BIN}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

    # 6. 启动服务
    ssh $SSH_OPTS "${SSH_USER}@${ip}" "chmod +x /usr/local/bin/${AGENT_BIN} && systemctl daemon-reload && systemctl enable --now lab-agent" 2>/dev/null
    echo "    [OK] $ip 部署完成"
}

# 并发扫描网段部署
for i in $(seq 1 254); do
    deploy_one "${SUBNET}.${i}" &
    # 限制并发数
    [ $((i % 20)) -eq 0 ] && wait
done
wait

echo "======================================================"
echo " 部署完成。在管理端 Agent 在线页面查看注册情况。"
echo "======================================================"
