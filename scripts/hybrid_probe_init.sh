#!/bin/sh
# ============================================================
# 机房教学管理系统 - 混合模式本地盘探测脚本 (V1.0-13)
# 运行环境: PXE 引导后的最小 initramfs / 探测系统
# 逻辑: 检测本地盘上的教学系统分区是否有效
#   - 有效 -> pivot 到本地盘启动
#   - 无效 -> 继续纯无盘启动（回退）
# 用法: 由 iPXE kernel 参数 lab_hybrid=1 触发，init 阶段调用
# ============================================================

LOCAL_LABEL="LAB-LOCAL"      # 本地系统分区标签
MARKER_FILE="/lab-teaching/.system-version"  # 有效性标记文件
MAX_BOOT_RETRY=3             # 历史启动失败阈值
STATE_DIR="/tmp/lab-hybrid"
mkdir -p "$STATE_DIR"

log() { echo "[hybrid] $1"; }

# 1. 检查是否处于混合模式
if [ "$lab_hybrid" != "1" ]; then
    log "非混合模式，跳过本地盘探测"
    exit 0
fi

# 2. 扫描本地分区，查找教学系统分区
FOUND_DEV=""
for dev in /dev/sda* /dev/nvme0n1p* /dev/mmcblk0p*; do
    [ -b "$dev" ] || continue
    LABEL=$(blkid -s LABEL -o value "$dev" 2>/dev/null)
    if [ "$LABEL" = "$LOCAL_LABEL" ]; then
        FOUND_DEV="$dev"
        break
    fi
done

if [ -z "$FOUND_DEV" ]; then
    log "未找到本地教学系统分区，回退无盘模式"
    exit 0
fi
log "发现本地分区: $FOUND_DEV"

# 3. 挂载并校验有效性标记
mkdir -p /mnt/local
if ! mount -o ro "$FOUND_DEV" /mnt/local 2>/dev/null; then
    log "本地分区挂载失败（文件系统损坏？），回退无盘模式"
    exit 0
fi

if [ ! -f "/mnt/local$MARKER_FILE" ]; then
    log "本地系统缺少版本标记文件，视为无效系统，回退无盘模式"
    umount /mnt/local
    exit 0
fi

LOCAL_VER=$(cat "/mnt/local$MARKER_FILE" 2>/dev/null)
log "本地系统版本: $LOCAL_VER"

# 4. 检查历史启动失败次数（防止坏本地盘反复循环）
RETRY_COUNT=$(cat "$STATE_DIR/retry_count" 2>/dev/null || echo 0)
if [ "$RETRY_COUNT" -ge "$MAX_BOOT_RETRY" ]; then
    log "本地盘连续 $RETRY_COUNT 次启动失败，强制回退无盘模式"
    umount /mnt/local
    exit 0
fi

# 5. 通过校验 -> 切换到本地盘启动
log "本地盘校验通过，切换到本地系统启动"
# 上报探测结果到管理端（best effort）
if command -v wget >/dev/null 2>&1; then
    SERVER_IP="${lab_server:-127.0.0.1}"
    wget -q -O /dev/null --timeout=3 \
        "http://$SERVER_IP:8090/api/v1/boot/report?mac=$(cat /sys/class/net/eth0/address 2>/dev/null)&mode=local&ver=$LOCAL_VER" 2>/dev/null || true
fi

# 交还本地引导（iPXE 环境执行 sanboot / kexec，具体由引导环境适配）
umount /mnt/local
if command -v kexec >/dev/null 2>&1 && [ -f "/mnt/local/boot/vmlinuz" ]; then
    log "kexec 切换本地内核"
    mount "$FOUND_DEV" /mnt/local
    kexec -l "/mnt/local/boot/vmlinuz" --initrd="/mnt/local/boot/initrd.img" --append="root=$FOUND_DEV ro quiet"
    kexec -e
fi
log "请引导环境执行本地启动（sanboot/exit）"
exit 0
