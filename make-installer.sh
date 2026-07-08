#!/usr/bin/env bash
# ============================================================
#  GridSim 自解压安装包生成器
#
#  用法：
#    bash make-installer.sh [tar.gz包路径]
#    bash make-installer.sh --all       # 生成 amd64 + arm64 两个安装包
#    bash make-installer.sh --arch arm64   # 指定架构
#
#  示例：
#    bash make-installer.sh dist/gridsim-v3.2.0-dev-14.e723e61-linux-amd64.tar.gz
#    bash make-installer.sh --all       # 自动从 dist/ 选择最新包，生成双架构
#    bash make-installer.sh             # 只生成 amd64（默认）
#
#  输出：
#    gridsim-install-v{version}-{arch}.sh  （可执行的自解压安装脚本）
#
#  使用生成的安装包：
#    scp gridsim-install-v3.2.0-linux-amd64.sh user@newserver:/tmp/
#    ssh user@newserver "bash /tmp/gridsim-install-v3.2.0-linux-amd64.sh"
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DIST_DIR="${SCRIPT_DIR}/dist"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
err()   { echo -e "${RED}[ERR]${NC} $*" >&2; exit 1; }

# ─── 生成单个架构的安装包 ─────────────────────────────────────────────────
generate_installer() {
    local PKG_PATH="$1"
    local PKG_NAME=$(basename "$PKG_PATH")

    # 提取版本号和架构
    local VERSION=$(echo "$PKG_NAME" | sed -n 's/gridsim-v\(.*\)-linux-\(amd64\|arm64\)\.tar\.gz/\1/p')
    local ARCH=$(echo "$PKG_NAME" | sed -n 's/gridsim-v.*-linux-\(amd64\|arm64\)\.tar\.gz/\1/p')
    [ -z "$VERSION" ] && err "无法从文件名提取版本号: $PKG_NAME"
    [ -z "$ARCH" ] && err "无法从文件名提取架构: $PKG_NAME"

    local OUTPUT="gridsim-install-v${VERSION}-linux-${ARCH}.sh"
    local PKG_SIZE=$(stat -c%s "$PKG_PATH" 2>/dev/null || stat -f%z "$PKG_PATH" 2>/dev/null)

    info "源包: $PKG_NAME ($(numfmt --to=iec $PKG_SIZE 2>/dev/null || echo "${PKG_SIZE} bytes"))"
    info "版本: $VERSION | 架构: $ARCH"
    info "生成: $OUTPUT"

# 生成自解压脚本头部
cat > "$OUTPUT" << 'INSTALLER_HEAD'
#!/usr/bin/env bash
# ============================================================
#  GridSim 自解压安装包
#  生成时间：__BUILD_TIME__
#  版本：__VERSION__
#
#  用法：
#    bash __FILENAME__              部署到默认目录
#    bash __FILENAME__ --target /opt/gridsim   指定部署目录
#    bash __FILENAME__ --extract-only          仅解压不部署
#    bash __FILENAME__ --status                查看当前服务状态
#    bash __FILENAME__ --stop                  停止服务
#    bash __FILENAME__ --start                 启动服务
#    bash __FILENAME__ --restart               重启服务
#
#  此文件是自包含的，只需一个文件即可完成部署。
# ============================================================
set -euo pipefail

# ─── 配置 ─────────────────────────────────────────────────────────────────
DEPLOY_DIR="${GRIDSIM_DEPLOY_DIR:-/home/envuser/IEC/gridsim}"
BACKUP_DIR="${DEPLOY_DIR}/backups"
SERVICE_NAME="gridsim"
API_PORT="${GRIDSIM_PORT:-8989}"
API_URL="http://localhost:${API_PORT}/api/v1/status"
ARCHIVE_MARKER="__ARCHIVE_BELOW__"
VERSION="__VERSION__"

# ─── 颜色 ─────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERR]${NC} $*" >&2; exit 1; }
step()  { echo -e "${CYAN}[$1/$TOTAL_STEPS]${NC} $2"; }

fmt_size() {
    local bytes=$1
    if [ "$bytes" -lt 1024 ]; then echo "${bytes}B"
    elif [ "$bytes" -lt 1048576 ]; then echo "$(( bytes / 1024 ))KB"
    elif [ "$bytes" -lt 1073741824 ]; then echo "$(( bytes / 1048576 ))MB"
    else echo "$(( bytes / 1073741824 ))GB"; fi
}

# ─── 从自身提取压缩包 ─────────────────────────────────────────────────────
extract_archive() {
    local target="$1"
    local archive_line
    archive_line=$(grep -n "^${ARCHIVE_MARKER}$" "$0" | tail -1 | cut -d: -f1)
    [ -z "$archive_line" ] && err "无法找到内嵌的压缩包数据"
    tail -n +$((archive_line + 1)) "$0" | tar xzf - -C "$target" --strip-components=1
}

# ─── 服务控制 ──────────────────────────────────────────────────────────────
do_stop() {
    info "停止 ${SERVICE_NAME} 服务..."
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    if [ -x "${DEPLOY_DIR}/bin/stop.sh" ]; then
        bash "${DEPLOY_DIR}/bin/stop.sh" 2>/dev/null || true
    fi
    pkill -x "$SERVICE_NAME" 2>/dev/null || true
    pkill -f "${SERVICE_NAME} serve" 2>/dev/null || true
    sleep 1
    if pgrep -x "$SERVICE_NAME" >/dev/null 2>&1; then
        pkill -9 -x "$SERVICE_NAME" 2>/dev/null || true
        sleep 1
    fi
    info "服务已停止"
}

do_start() {
    info "启动 ${SERVICE_NAME} 服务..."
    if systemctl start "$SERVICE_NAME" 2>/dev/null; then
        info "  systemctl start 成功"
        return 0
    fi
    if [ -x "${DEPLOY_DIR}/bin/start.sh" ]; then
        if bash "${DEPLOY_DIR}/bin/start.sh" 2>/dev/null; then
            info "  start.sh 启动成功"
            return 0
        fi
    fi
    # 回退：直接启动
    mkdir -p "${DEPLOY_DIR}/logs"
    cd "$DEPLOY_DIR"
    nohup "${DEPLOY_DIR}/bin/${SERVICE_NAME}" serve \
        --http ":${API_PORT}" \
        --config-dir "${DEPLOY_DIR}/config" \
        --log-dir "${DEPLOY_DIR}/logs" \
        --log info \
        > "${DEPLOY_DIR}/logs/output.log" 2>&1 &
    echo $! > "${DEPLOY_DIR}/logs/pid"
    info "  直接启动成功 (PID: $!)"
}

do_restart() {
    do_stop
    sleep 1
    do_start
}

# ─── systemd 服务注册 ─────────────────────────────────────────────────────
ensure_systemd() {
    local unit_path="/etc/systemd/system/${SERVICE_NAME}.service"
    if [ -f "$unit_path" ]; then
        # 更新 WorkingDirectory（如果部署目录变了）
        if ! grep -q "$DEPLOY_DIR" "$unit_path" 2>/dev/null; then
            sed -i "s|WorkingDirectory=.*|WorkingDirectory=${DEPLOY_DIR}|" "$unit_path"
            sed -i "s|ExecStart=.*|ExecStart=/bin/bash ${DEPLOY_DIR}/bin/start.sh|" "$unit_path"
            sed -i "s|ExecStop=.*|ExecStop=/bin/bash ${DEPLOY_DIR}/bin/stop.sh|" "$unit_path"
            sed -i "s|PIDFile=.*|PIDFile=${DEPLOY_DIR}/logs/pid|" "$unit_path"
            systemctl daemon-reload 2>/dev/null || true
            info "  systemd 单元路径已更新"
        fi
        return 0
    fi

    info "  创建 systemd 服务单元..."
    cat > "$unit_path" << UNIT
[Unit]
Description=Grid Simulator
After=network.target

[Service]
Type=forking
User=root
WorkingDirectory=${DEPLOY_DIR}
ExecStart=/bin/bash ${DEPLOY_DIR}/bin/start.sh
ExecStop=/bin/bash ${DEPLOY_DIR}/bin/stop.sh
PIDFile=${DEPLOY_DIR}/logs/pid
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT
    systemctl daemon-reload 2>/dev/null || true
    systemctl enable "$SERVICE_NAME" 2>/dev/null || true
    info "  systemd 服务已注册"
}

# ─── 健康检查 ──────────────────────────────────────────────────────────────
health_check() {
    info "健康检查..."
    local ok=false
    for i in $(seq 1 5); do
        local http_code
        http_code=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL" 2>/dev/null || echo "000")
        if [ "$http_code" = "200" ]; then
            ok=true
            break
        fi
        [ "$i" -lt 5 ] && sleep 3
    done

    if [ "$ok" = true ]; then
        info "  ✅ API: HTTP 200 OK"
        local status_json
        status_json=$(curl -s "$API_URL" 2>/dev/null || echo "{}")
        local running_ver=$(echo "$status_json" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
        [ -n "$running_ver" ] && info "  ✅ 运行版本: $running_ver"
        local instances
        instances=$(curl -s "http://localhost:${API_PORT}/api/v1/instances" 2>/dev/null || echo "[]")
        local inst_count=$(echo "$instances" | grep -o '"id"' | wc -l)
        info "  ✅ 实例数: $inst_count"
        return 0
    else
        warn "  ⚠ API 未返回 200（服务可能需要更多启动时间）"
        return 1
    fi
}

# ─── 查看状态 ──────────────────────────────────────────────────────────────
show_status() {
    echo "═══════════════════════════════════════════════════════════════"
    echo "  GridSim 服务状态"
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        info "systemd: active"
    else
        warn "systemd: inactive"
    fi
    local pid_file="${DEPLOY_DIR}/logs/pid"
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            info "PID: $pid (running)"
        else
            warn "PID: $pid (进程不存在)"
        fi
    fi
    local http_code
    http_code=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL" 2>/dev/null || echo "000")
    if [ "$http_code" = "200" ]; then
        info "API: HTTP 200 OK"
        local status_json
        status_json=$(curl -s "$API_URL" 2>/dev/null || echo "{}")
        local running_ver=$(echo "$status_json" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
        [ -n "$running_ver" ] && info "运行版本: $running_ver"
    else
        warn "API: HTTP $http_code"
    fi
    local ver_file="${DEPLOY_DIR}/bin/VERSION"
    [ -f "$ver_file" ] && info "部署版本: $(cat "$ver_file")"
    echo ""
}

# ─── 部署主流程 ────────────────────────────────────────────────────────────
do_deploy() {
    local TOTAL_STEPS=7

    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo "  GridSim 自解压安装"
    echo "═══════════════════════════════════════════════════════════════"
    echo "  版本:   v${VERSION}"
    echo "  部署到: ${DEPLOY_DIR}"
    echo "═══════════════════════════════════════════════════════════════"
    echo ""

    # 1. 前置检查
    step 1 "前置检查"
    mkdir -p "$DEPLOY_DIR"
    local avail=$(df --output=avail "$DEPLOY_DIR" 2>/dev/null | tail -1 || echo "999999999")
    avail=$((avail * 1024))
    info "  磁盘可用: $(fmt_size $avail)"

    local is_upgrade=false
    if [ -f "${DEPLOY_DIR}/bin/start.sh" ] || [ -f "${DEPLOY_DIR}/bin/${SERVICE_NAME}" ]; then
        is_upgrade=true
        info "  模式: 🔄 升级"
    else
        info "  模式: 🆕 全新部署"
    fi

    # 2. 停止服务
    step 2 "停止服务"
    if [ "$is_upgrade" = true ]; then
        do_stop
    else
        info "  (全新部署，跳过)"
    fi

    # 3. 备份配置
    step 3 "备份配置"
    if [ "$is_upgrade" = true ] && [ -d "${DEPLOY_DIR}/config" ] && [ "$(ls -A "${DEPLOY_DIR}/config" 2>/dev/null)" ]; then
        local ts=$(date '+%Y%m%d-%H%M%S')
        local backup_path="${BACKUP_DIR}/config-pre-upgrade-${ts}"
        mkdir -p "$backup_path"
        cp -r "${DEPLOY_DIR}/config/"* "$backup_path/" 2>/dev/null || true
        echo "{\"backup_time\":\"$ts\",\"version\":\"$(cat "${DEPLOY_DIR}/bin/VERSION" 2>/dev/null || echo unknown)\"}" \
            > "$backup_path/manifest.json"
        info "  已备份 → config-pre-upgrade-${ts}"
    else
        info "  (无需备份)"
    fi

    # 4. 清理旧文件
    step 4 "清理旧文件"
    if [ "$is_upgrade" = true ]; then
        for item in "${DEPLOY_DIR}"/*; do
            [ "$(basename "$item")" = "backups" ] && continue
            rm -rf "$item"
        done
        info "  已清理（保留 backups）"
    else
        info "  (跳过)"
    fi

    # 5. 解压
    step 5 "解压部署"
    mkdir -p "$DEPLOY_DIR"
    extract_archive "$DEPLOY_DIR"
    chmod +x "${DEPLOY_DIR}/bin/"* 2>/dev/null || true
    mkdir -p "${DEPLOY_DIR}/config" "${DEPLOY_DIR}/logs"
    info "  解压完成"
    info "  版本: $(cat "${DEPLOY_DIR}/bin/VERSION" 2>/dev/null || echo "$VERSION")"

    # 6. 恢复配置
    step 6 "恢复配置"
    if [ "$is_upgrade" = true ] && [ -d "$BACKUP_DIR" ]; then
        local latest_backup=$(ls -d "${BACKUP_DIR}"/config-pre-upgrade-* 2>/dev/null | sort -r | head -1)
        if [ -n "$latest_backup" ] && [ -d "$latest_backup" ]; then
            cp -r "$latest_backup"/* "${DEPLOY_DIR}/config/" 2>/dev/null || true
            rm -f "${DEPLOY_DIR}/config/manifest.json"
            info "  已恢复 (from $(basename "$latest_backup"))"
        else
            info "  (无备份可恢复)"
        fi
    else
        info "  (跳过)"
    fi

    # 7. 注册服务 + 启动 + 验证
    step 7 "启动服务"
    ensure_systemd
    do_start
    sleep 2
    health_check

    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    info "🎉 部署完成! v${VERSION}"
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
    echo "  管理命令:"
    echo "    systemctl status  ${SERVICE_NAME}"
    echo "    systemctl restart ${SERVICE_NAME}"
    echo "    systemctl stop    ${SERVICE_NAME}"
    echo "    tail -f ${DEPLOY_DIR}/logs/output.log"
    echo ""
    echo "  Web UI: http://localhost:${API_PORT}"
    local host_ip=$(hostname -I 2>/dev/null | awk '{print $1}')
    [ -n "$host_ip" ] && echo "  局域网: http://${host_ip}:${API_PORT}"
    echo ""
}

# ─── 入口 ──────────────────────────────────────────────────────────────────
case "${1:-}" in
    --extract-only)
        TARGET="${2:-.}"
        info "仅解压到: $TARGET"
        mkdir -p "$TARGET"
        extract_archive "$TARGET"
        info "解压完成"
        ;;
    --target)
        [ -z "${2:-}" ] && err "请指定目标目录: bash $0 --target /path/to/dir"
        DEPLOY_DIR="$2"
        BACKUP_DIR="${DEPLOY_DIR}/backups"
        do_deploy
        ;;
    --status|-s)
        show_status
        ;;
    --stop)
        do_stop
        ;;
    --start)
        do_start
        sleep 2
        health_check
        ;;
    --restart)
        do_restart
        sleep 2
        health_check
        ;;
    --help|-h)
        echo "GridSim 自解压安装包 v${VERSION}"
        echo ""
        echo "用法:"
        echo "  bash $0                     部署到默认目录 (${DEPLOY_DIR})"
        echo "  bash $0 --target <dir>      部署到指定目录"
        echo "  bash $0 --extract-only      仅解压到当前目录"
        echo "  bash $0 --status            查看服务状态"
        echo "  bash $0 --stop              停止服务"
        echo "  bash $0 --start             启动服务"
        echo "  bash $0 --restart           重启服务"
        echo ""
        echo "环境变量:"
        echo "  GRIDSIM_DEPLOY_DIR   部署目录 (默认: /home/envuser/IEC/gridsim)"
        echo "  GRIDSIM_PORT         API 端口 (默认: 8989)"
        ;;
    "")
        do_deploy
        ;;
    *)
        err "未知参数: $1（使用 --help 查看帮助）"
        ;;
esac

exit 0
__ARCHIVE_BELOW__
INSTALLER_HEAD

# 替换占位符
BUILD_TIME=$(date '+%Y-%m-%d %H:%M:%S')
sed -i "s|__VERSION__|${VERSION}|g" "$OUTPUT"
sed -i "s|__BUILD_TIME__|${BUILD_TIME}|g" "$OUTPUT"
sed -i "s|__FILENAME__|${OUTPUT}|g" "$OUTPUT"

# 追加 tar.gz 二进制数据
cat "$PKG_PATH" >> "$OUTPUT"

# 设置可执行权限
chmod +x "$OUTPUT"

# 输出结果
OUTPUT_SIZE=$(stat -c%s "$OUTPUT" 2>/dev/null || stat -f%z "$OUTPUT" 2>/dev/null)
echo ""
info "  ✔ ${OUTPUT} ($(numfmt --to=iec $OUTPUT_SIZE 2>/dev/null || echo "${OUTPUT_SIZE} bytes"))"
echo ""
}

# ─── 入口 ──────────────────────────────────────────────────────────────────
ALL_ARCHS=false
TARGET_ARCH=""
PKG_ARG=""

for arg in "$@"; do
    case "$arg" in
        --all|-a)    ALL_ARCHS=true ;;
        --arch)      shift; TARGET_ARCH="${1:-}" ;;
        --help|-h)
            echo "GridSim 自解压安装包生成器"
            echo ""
            echo "用法:"
            echo "  bash $0                           生成 amd64 安装包（默认）"
            echo "  bash $0 --all                     生成 amd64 + arm64 两个安装包"
            echo "  bash $0 --arch arm64              指定架构"
            echo "  bash $0 <file.tar.gz>             指定包文件"
            echo ""
            exit 0
            ;;
        *)           PKG_ARG="$arg" ;;
    esac
done

if [ -n "$PKG_ARG" ] && [ -f "$PKG_ARG" ]; then
    # 直接指定包文件
    generate_installer "$PKG_ARG"
elif [ "$ALL_ARCHS" = true ]; then
    # 生成 amd64 + arm64
    echo ""
    info "═══════════════════════════════════════════════════════════════"
    info "  生成双架构安装包"
    info "═══════════════════════════════════════════════════════════════"
    echo ""
    GENERATED=0
    for arch in amd64 arm64; do
        PKG=$(ls -t "${DIST_DIR}"/gridsim-v*-linux-${arch}.tar.gz 2>/dev/null | head -1)
        if [ -n "$PKG" ]; then
            generate_installer "$PKG"
            GENERATED=$((GENERATED + 1))
        else
            echo -e "${YELLOW}[WARN]${NC} 未找到 linux-${arch} 包，跳过"
        fi
    done
    echo ""
    info "═══════════════════════════════════════════════════════════════"
    info "  完成! 共生成 ${GENERATED} 个安装包"
    info "═══════════════════════════════════════════════════════════════"
    ls -lh gridsim-install-*.sh 2>/dev/null | awk '{printf "  %s  %s\n", $5, $9}'
    echo ""
elif [ -n "$TARGET_ARCH" ]; then
    # 指定架构
    PKG=$(ls -t "${DIST_DIR}"/gridsim-v*-linux-${TARGET_ARCH}.tar.gz 2>/dev/null | head -1)
    [ -z "$PKG" ] && err "未找到 linux-${TARGET_ARCH} 包"
    generate_installer "$PKG"
else
    # 默认：只生成 amd64
    PKG=$(ls -t "${DIST_DIR}"/gridsim-v*-linux-amd64.tar.gz 2>/dev/null | head -1)
    [ -z "$PKG" ] && err "未找到安装包。用法: bash $0 <gridsim-v*-linux-amd64.tar.gz> 或 bash $0 --all"
    generate_installer "$PKG"
fi
