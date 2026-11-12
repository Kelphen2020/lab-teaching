#!/usr/bin/env bash
# ============================================================
# 机房教学管理系统 - 无盘 PXE/iPXE 启动环境验证脚本
# 用途: 在部署无盘启动前检查服务器是否满足 PXE 引导条件
# 适用: Ubuntu/Debian/CentOS + 国产 OS（麒麟/统信）
# 用法: sudo bash scripts/check_pxe_env.sh
# ============================================================
set -u

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
pass() { echo -e "  ${GREEN}[OK]${NC} $1"; }
fail() { echo -e "  ${RED}[FAIL]${NC} $1"; }
warn() { echo -e "  ${YELLOW}[WARN]${NC} $1"; }
section() { echo -e "\n${YELLOW}== $1 ==${NC}"; }

echo "======================================================"
echo " 无盘 PXE 启动环境检查"
echo " 主机: $(hostname)  IP: $(hostname -I 2>/dev/null | awk '{print $1}')"
echo "======================================================"

# 1.  root 权限
section "权限检查"
if [ "$(id -u)" -eq 0 ]; then pass "以 root 运行"; else fail "需要 root 权限 (sudo)"; fi

# 2.  DHCP 服务
section "DHCP 服务 (PXE 客户端获取 IP + next-server)"
if command -v dhcpd >/dev/null 2>&1; then
    pass "已安装 isc-dhcp-server: $(dhcpd --version 2>&1 | head -1)"
elif command -v dnsmasq >/dev/null 2>&1; then
    pass "已安装 dnsmasq (可做轻量 DHCP+TFTP): $(dnsmasq --version | head -1)"
else
    warn "未安装 DHCP 服务，建议: apt install isc-dhcp-server 或 dnsmasq"
fi

# 3.  TFTP 服务
section "TFTP 服务 (下发 iPXE 引导文件)"
if command -v in.tftpd >/dev/null 2>&1 || command -v tftpd-hpa >/dev/null 2>&1; then
    pass "已安装 tftpd-hpa"
elif systemctl list-unit-files 2>/dev/null | grep -q dnsmasq; then
    pass "dnsmasq 可内置 TFTP"
else
    warn "未安装 TFTP，建议: apt install tftpd-hpa"
fi

# 4.  iPXE 引导文件
section "iPXE 引导文件"
TFTP_ROOT="/srv/tftp"
[ -d /var/lib/tftpboot ] && TFTP_ROOT="/var/lib/tftpboot"
if ls "$TFTP_ROOT"/*.efi "$TFTP_ROOT"/undionly.kpxe 2>/dev/null | grep -q .; then
    pass "TFTP 根目录 $TFTP_ROOT 存在引导文件:"
    ls -1 "$TFTP_ROOT" 2>/dev/null | sed 's/^/       /'
else
    warn "TFTP 根目录 $TFTP_ROOT 暂无 iPXE 引导文件 (undionly.kpxe / ipxe.efi)"
    echo "       下载: wget https://boot.ipxe.org/undionly.kpxe"
    echo "             wget https://boot.ipxe.org/ipxe.efi"
fi

# 5.  HTTP 镜像仓库 (存放系统镜像 rootfs)
section "HTTP 镜像仓库 (iSCSI/NFS rootfs 或镜像下载)"
if command -v nginx >/dev/null 2>&1; then
    pass "已安装 nginx，可用于托管系统镜像"
else
    warn "未安装 nginx，建议安装用于托管无盘系统镜像"
fi

# 6.  关键端口监听
section "关键端口监听状态"
for port in 67 69 80 443; do
    if ss -tuln 2>/dev/null | grep -qE ":$port\b"; then
        pass "端口 $port 已监听"
    else
        warn "端口 $port 未监听 ($( [ $port = 67 ] && echo DHCP || ([ $port = 69 ] && echo TFTP || echo HTTP) ))"
    fi
done

# 7.  防火墙
section "防火墙状态"
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "active"; then
    warn "ufw 已启用，需放行 67/69/80/443 及 iSCSI 3260 端口"
else
    pass "ufw 未启用或不存在（生产环境建议配置放行规则）"
fi

# 8.  iSCSI target (无盘根文件系统)
section "iSCSI Target (无盘系统盘)"
if command -v tgtadm >/dev/null 2>&1 || command -v targetcli >/dev/null 2>&1; then
    pass "已安装 iSCSI target 工具"
else
    warn "未安装 iSCSI target，建议: apt install tgt 或 targetcli-fb"
fi

echo ""
echo "======================================================"
echo " 检查完成。无盘启动链路: "
echo "   学生机 PXE ROM -> DHCP(67) 获取IP+next-server"
echo "   -> TFTP(69) 下载 iPXE -> HTTP(80) 加载引导脚本"
echo "   -> iSCSI(3260)/NFS 挂载无盘根文件系统 -> 启动 Agent"
echo "======================================================"
