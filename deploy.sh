#!/bin/bash
# 机房教学管理系统 - 一键部署脚本
# 用法: ./deploy.sh [install|start|stop|status|update]
set -e

APP_NAME="lab-teaching"
INSTALL_DIR="/opt/lab-teaching"
BACKUP_DIR="/opt/lab-teaching/backups"
LOG_FILE="/var/log/lab-teaching/deploy.log"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1" | tee -a "$LOG_FILE"; }
err() { echo -e "${RED}[ERROR]${NC} $1" | tee -a "$LOG_FILE"; exit 1; }

check_root() {
    if [ "$EUID" -ne 0 ]; then
        err "请使用 root 权限运行: sudo $0 $1"
    fi
}

check_deps() {
    log "检查系统依赖..."
    local missing=()
    command -v docker >/dev/null 2>&1 || missing+=("docker")
    command -v docker compose version >/dev/null 2>&1 || missing+=("docker-compose-plugin")
    
    if [ ${#missing[@]} -gt 0 ]; then
        warn "缺少依赖: ${missing[*]}"
        read -p "是否自动安装? (y/n): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            for dep in "${missing[@]}"; do
                case $dep in
                    docker)
                        log "安装 Docker..."
                        curl -fsSL https://get.docker.com | sh
                        systemctl enable docker
                        systemctl start docker
                        ;;
                    docker-compose-plugin)
                        log "安装 Docker Compose 插件..."
                        apt-get install -y docker-compose-plugin 2>/dev/null || yum install -y docker-compose-plugin
                        ;;
                esac
            done
        else
            err "请先安装缺少的依赖"
        fi
    fi
    log "依赖检查通过 ✓"
}

do_install() {
    check_root "install"
    check_deps
    
    log "=== 机房教学管理系统 安装向导 ==="
    
    # 配置收集
    read -p "数据库密码 [postgres]: " DB_PASSWORD
    DB_PASSWORD=${DB_PASSWORD:-postgres}
    
    read -p "JWT密钥 [自动生成]: " JWT_SECRET
    if [ -z "$JWT_SECRET" ]; then
        JWT_SECRET=$(openssl rand -hex 32)
        log "已自动生成JWT密钥"
    fi
    
    read -p "服务端口 [8090]: " API_PORT
    API_PORT=${API_PORT:-8090}
    
    read -p "前端端口 [80]: " WEB_PORT
    WEB_PORT=${WEB_PORT:-80}
    
    # 创建目录
    mkdir -p "$INSTALL_DIR" "$BACKUP_DIR" /var/log/lab-teaching
    
    # 复制文件
    log "部署文件到 $INSTALL_DIR ..."
    cp -r ./* "$INSTALL_DIR/" 2>/dev/null || true
    
    # 写入环境变量
    cat > "$INSTALL_DIR/.env" << EOF
DB_PASSWORD=$DB_PASSWORD
JWT_SECRET=$JWT_SECRET
API_PORT=$API_PORT
WEB_PORT=$WEB_PORT
EOF
    chmod 600 "$INSTALL_DIR/.env"
    
    # 更新 docker-compose 端口映射
    if [ "$API_PORT" != "8090" ] || [ "$WEB_PORT" != "80" ]; then
        sed -i "s/8090:8090/$API_PORT:8090/" "$INSTALL_DIR/docker-compose.yml"
        sed -i "s/80:80/$WEB_PORT:80/" "$INSTALL_DIR/docker-compose.yml"
    fi
    
    log "启动服务..."
    cd "$INSTALL_DIR"
    docker compose up -d --build
    
    # 等待健康检查
    log "等待服务就绪..."
    for i in $(seq 1 30); do
        if curl -s "http://localhost:$API_PORT/health" | grep -q "ok"; then
            log "服务启动成功 ✓"
            log "管理后台: http://$(hostname -I | awk '{print $1}'):$WEB_PORT"
            log "API地址: http://$(hostname -I | awk '{print $1}'):$API_PORT"
            log "默认账号: admin / ChangeMe@123 (可用 LAB_ADMIN_INIT_PASSWORD 覆盖)"
            return 0
        fi
        sleep 2
    done
    err "服务启动超时，请检查日志: docker compose logs"
}

do_start() {
    cd "$INSTALL_DIR"
    docker compose up -d
    log "服务已启动"
}

do_stop() {
    cd "$INSTALL_DIR"
    docker compose down
    log "服务已停止"
}

do_status() {
    cd "$INSTALL_DIR"
    docker compose ps
    echo ""
    curl -s "http://localhost:${API_PORT:-8090}/health" | python3 -m json.tool 2>/dev/null || echo "API 未响应"
}

do_update() {
    check_root "update"
    log "=== 更新部署 ==="
    
    # 备份数据库
    log "备份数据库..."
    bash "$INSTALL_DIR/scripts/backup.sh"
    
    # 拉取新代码
    cp -r ./* "$INSTALL_DIR/" 2>/dev/null || true
    
    # 重建并重启
    cd "$INSTALL_DIR"
    docker compose up -d --build
    log "更新完成 ✓"
}

# 主入口
case "${1:-}" in
    install) do_install ;;
    start)   do_start ;;
    stop)    do_stop ;;
    status)  do_status ;;
    update)  do_update ;;
    *)
        echo "用法: $0 {install|start|stop|status|update}"
        echo ""
        echo "  install  - 首次安装（交互式配置）"
        echo "  start    - 启动服务"
        echo "  stop     - 停止服务"
        echo "  status   - 查看状态"
        echo "  update   - 更新部署（自动备份）"
        exit 1
        ;;
esac
