#!/bin/bash
# 机房教学管理系统 - PostgreSQL 备份与恢复脚本
# 用法: ./backup.sh [backup|restore|list|auto]
set -e

BACKUP_DIR="${BACKUP_DIR:-/opt/lab-teaching/backups}"
DB_NAME="lab_teaching"
DB_USER="postgres"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
KEEP_DAYS=30  # 保留最近30天的备份

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

log() { echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"; }
err() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

mkdir -p "$BACKUP_DIR"

do_backup() {
    local timestamp=$(date '+%Y%m%d_%H%M%S')
    local backup_file="$BACKUP_DIR/${DB_NAME}_${timestamp}.sql.gz"
    
    log "开始备份数据库 $DB_NAME ..."
    
    # 检测运行方式（Docker 或本地）
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "lab-teaching-db"; then
        # Docker 环境
        docker exec lab-teaching-db pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$backup_file"
    else
        # 本地 PostgreSQL
        PGPASSWORD="${DB_PASSWORD:-postgres}" pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$DB_NAME" | gzip > "$backup_file"
    fi
    
    local size=$(du -h "$backup_file" | cut -f1)
    log "备份完成: $backup_file ($size)"
    
    # 清理过期备份
    local deleted=$(find "$BACKUP_DIR" -name "*.sql.gz" -mtime +$KEEP_DAYS -delete -print | wc -l)
    if [ "$deleted" -gt 0 ]; then
        log "已清理 $deleted 个过期备份(>${KEEP_DAYS}天)"
    fi
    
    echo "$backup_file"
}

do_restore() {
    local backup_file="$1"
    
    if [ -z "$backup_file" ]; then
        # 自动选择最新备份
        backup_file=$(ls -t "$BACKUP_DIR"/*.sql.gz 2>/dev/null | head -1)
        if [ -z "$backup_file" ]; then
            err "未找到备份文件，请先执行 backup"
        fi
        log "使用最新备份: $backup_file"
    fi
    
    if [ ! -f "$backup_file" ]; then
        err "备份文件不存在: $backup_file"
    fi
    
    log "⚠️  即将恢复数据库 $DB_NAME，当前数据将被覆盖！"
    read -p "确认恢复? (输入 YES 继续): " confirm
    if [ "$confirm" != "YES" ]; then
        log "已取消"
        return
    fi
    
    log "开始恢复..."
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "lab-teaching-db"; then
        gunzip -c "$backup_file" | docker exec -i lab-teaching-db psql -U "$DB_USER" -d "$DB_NAME"
    else
        gunzip -c "$backup_file" | PGPASSWORD="${DB_PASSWORD:-postgres}" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME"
    fi
    
    log "恢复完成 ✓"
}

do_list() {
    echo "=== 备份文件列表 ($BACKUP_DIR) ==="
    echo ""
    if ls "$BACKUP_DIR"/*.sql.gz >/dev/null 2>&1; then
        printf "%-45s %8s %s\n" "文件名" "大小" "时间"
        printf "%-45s %8s %s\n" "------" "----" "----"
        for f in $(ls -t "$BACKUP_DIR"/*.sql.gz); do
            local name=$(basename "$f")
            local size=$(du -h "$f" | cut -f1)
            local mtime=$(stat -c '%y' "$f" 2>/dev/null | cut -d. -f1)
            printf "%-45s %8s %s\n" "$name" "$size" "$mtime"
        done
        echo ""
        echo "共 $(ls "$BACKUP_DIR"/*.sql.gz | wc -l) 个备份"
    else
        echo "暂无备份文件"
    fi
}

do_auto() {
    # 安装 crontab 定时备份（每天凌晨2点）
    # 注意：cron 不支持行内 VAR=xxx 前缀，须用 bash -c 包裹
    mkdir -p "$BACKUP_DIR"
    local script_path
    script_path="$(realpath "$0")"
    local cron_cmd="0 2 * * * /bin/bash -c 'BACKUP_DIR=\"$BACKUP_DIR\" /bin/bash \"$script_path\" backup >> \"$BACKUP_DIR/backup.log\" 2>&1'"

    # 检查是否已存在
    if crontab -l 2>/dev/null | grep -q "lab-teaching.*backup"; then
        log "定时备份已配置"
        crontab -l | grep "lab-teaching"
        return
    fi

    (crontab -l 2>/dev/null | grep -v '^# (- installed' | grep -v '^# DO NOT EDIT' | grep -v '^# (Cron version'; echo "$cron_cmd") > /tmp/.lab_crontab.$$
    crontab /tmp/.lab_crontab.$$
    rm -f /tmp/.lab_crontab.$$
    if crontab -l | grep -q "lab-teaching.*backup"; then
        log "已配置定时备份: 每天凌晨 02:00 自动执行"
        log "日志: $BACKUP_DIR/backup.log"
        log "保留策略: 最近 $KEEP_DAYS 天"
        crontab -l | grep "lab-teaching"
    else
        echo "错误: crontab 安装失败，请手动添加: $cron_cmd" >&2
        exit 1
    fi
}

case "${1:-backup}" in
    backup)  do_backup ;;
    restore) do_restore "$2" ;;
    list)    do_list ;;
    auto)    do_auto ;;
    *)
        echo "用法: $0 {backup|restore [file]|list|auto}"
        echo ""
        echo "  backup        - 立即备份数据库"
        echo "  restore [file]- 恢复数据库（默认用最新备份）"
        echo "  list          - 列出所有备份"
        echo "  auto          - 配置每日自动备份(crontab)"
        exit 1
        ;;
esac
