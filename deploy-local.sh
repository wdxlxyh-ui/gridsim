#!/usr/bin/env bash
# ============================================================
#  GridSim 本地快速部署脚本
#  功能：将构建出的包部署到本机的模拟器运行位置
#
#  用法：
#    bash deploy-local.sh                   # 自动从 DIST_DIR 选择最新版本部署
#    bash deploy-local.sh <包路径>          # 直接指定 tar.gz 包部署（新环境推荐）
#    bash deploy-local.sh --version 3.1.0-dev-14.5ea5bbe  # 按版本号部署
#    bash deploy-local.sh --list            # 列出可用版本
#    bash deploy-local.sh --status          # 查看服务状态
#    bash deploy-local.sh --stop            # 仅停止服务
#    bash deploy-local.sh --start           # 仅启动服务
#    bash deploy-local.sh --restart         # 重启服务
#    bash deploy-local.sh --rollback        # 回滚到上次配置备份
#
#  新环境快速部署（仅需两个文件）：
#    1. 将 deploy-local.sh 和 gridsim-v*-linux-amd64.tar.gz 放到任意目录
#    2. 执行: bash deploy-local.sh ./gridsim-v3.1.0-dev-14.5ea5bbe-linux-amd64.tar.gz
#    脚本和包文件可以放在任意目录，没有路径要求。
#
#  路径约定：
#    构建产物目录（编译环境）：/root/IEC-SIM/iec104-sim-master/dist/
#    部署目标目录：/home/envuser/IEC/gridsim/（可通过 DEPLOY_DIR 环境变量覆盖）
#    systemd 服务：gridsim
# ============================================================
set -euo pipefail

# ─── 配置（可通过环境变量覆盖） ─────────────────────────────────────────
DIST_DIR="${GRIDSIM_DIST_DIR:-/root/IEC-SIM/iec104-sim-master/dist}"
DEPLOY_DIR="${GRIDSIM_DEPLOY_DIR:-/home/envuser/IEC/gridsim}"
BACKUP_DIR="${DEPLOY_DIR}/backups"
SERVICE_NAME="gridsim"
API_PORT="${GRIDSIM_PORT:-8989}"
API_URL="http://localhost:${API_PORT}/api/v1/status"

# 包名正则（匹配 linux-amd64）
PKG_GLOB="gridsim-v*-linux-amd64.tar.gz"

# ─── 颜色 ─────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERR]${NC} $*" >&2; }
step()  { echo -e "${CYAN}[$1/$TOTAL_STEPS]${NC} $2"; }

# ─── 工具函数 ─────────────────────────────────────────────────────────────
fmt_size() {
    local bytes=$1
    if [ "$bytes" -lt 1024 ]; then echo "${bytes}B"
    elif [ "$bytes" -lt 1048576 ]; then echo "$(( bytes / 1024 ))KB"
    elif [ "$bytes" -lt 1073741824 ]; then echo "$(( bytes / 1048576 ))MB"
    else echo "$(( bytes / 1073741824 ))GB"; fi
}

get_latest_pkg() {
    # 按修改时间排序取最新的 linux-amd64 包
    ls -t "${DIST_DIR}"/${PKG_GLOB} 2>/dev/null | head -1
}

extract_version() {
    # 从文件名提取版本号
    local filename="$1"
    echo "$filename" | sed -n 's/.*gridsim-v\(.*\)-linux-amd64\.tar\.gz$/\1/p'
}

list_versions() {
    echo "═══════════════════════════════════════════════════════════════"
    echo "  可用版本（${DIST_DIR}）"
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
    local count=0
    for f in $(ls -t "${DIST_DIR}"/${PKG_GLOB} 2>/dev/null); do
        local ver=$(extract_version "$(basename "$f")")
        local sz=$(stat -c%s "$f" 2>/dev/null || echo 0)
        local mtime=$(stat -c%y "$f" 2>/dev/null | cut -d'.' -f1)
        printf "  %-40s  %s  %s\n" "v${ver}" "$(fmt_size $sz)" "$mtime"
        count=$((count + 1))
    done
    if [ "$count" -eq 0 ]; then
        echo "  (无可用版本)"
    fi
    echo ""
    echo "  共 ${count} 个版本"
}

service_status() {
    echo "═══════════════════════════════════════════════════════════════"
    echo "  GridSim 服务状态"
    echo "═══════════════════════════════════════════════════════════════"
    echo ""

    # systemd 状态
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        info "systemd: ${GREEN}active${NC}"
    else
        warn "systemd: inactive"
    fi

    # PID 检查
    local pid_file="${DEPLOY_DIR}/logs/pid"
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            info "PID: $pid (running)"
            # 运行时间
            local etimes=$(ps -o etimes= -p "$pid" 2>/dev/null | tr -d ' ')
            if [ -n "$etimes" ]; then
                local days=$((etimes / 86400))
                local hours=$(( (etimes % 86400) / 3600 ))
                local mins=$(( (etimes % 3600) / 60 ))
                info "运行时间: ${days}d ${hours}h ${mins}m"
            fi
        else
            warn "PID: $pid (进程不存在)"
        fi
    else
        warn "PID 文件不存在"
    fi

    # API 检查
    local http_code
    http_code=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL" 2>/dev/null || echo "000")
    if [ "$http_code" = "200" ]; then
        info "API: HTTP 200 OK"
        # 获取版本
        local status_json
        status_json=$(curl -s "$API_URL" 2>/dev/null || echo "{}")
        local running_ver=$(echo "$status_json" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
        [ -n "$running_ver" ] && info "运行版本: $running_ver"
        # 实例数
        local instances
        instances=$(curl -s "http://localhost:${API_PORT}/api/v1/instances" 2>/dev/null || echo "[]")
        local inst_count=$(echo "$instances" | grep -o '"id"' | wc -l)
        info "实例数: $inst_count"
    else
        warn "API: HTTP $http_code (不可用)"
    fi

    # 端口检查
    echo ""
    info "端口监听:"
    ss -tlnp 2>/dev/null | grep -E "${API_PORT}|2404" | while read -r line; do
        echo "    $line"
    done

    # 当前部署版本
    local ver_file="${DEPLOY_DIR}/bin/VERSION"
    if [ -f "$ver_file" ]; then
        info "部署版本: $(cat "$ver_file")"
    fi
    echo ""
}

# ─── 服务控制 ──────────────────────────────────────────────────────────────
do_stop() {
    info "停止 ${SERVICE_NAME} 服务..."

    # 策略 1: systemctl
    if systemctl stop "$SERVICE_NAME" 2>/dev/null; then
        info "  systemctl stop 成功"
    else
        warn "  systemctl 不可用"
    fi

    # 策略 2: stop.sh 脚本
    if [ -x "${DEPLOY_DIR}/bin/stop.sh" ]; then
        bash "${DEPLOY_DIR}/bin/stop.sh" 2>/dev/null || true
    fi

    # 策略 3: pkill 清理残留进程
    pkill -x "$SERVICE_NAME" 2>/dev/null || true
    pkill -f "${DEPLOY_DIR}/bin/${SERVICE_NAME}" 2>/dev/null || true
    pkill -f "${SERVICE_NAME} serve" 2>/dev/null || true
    sleep 1

    # 验证
    if pgrep -x "$SERVICE_NAME" >/dev/null 2>&1; then
        warn "进程仍在运行，强制 kill..."
        pkill -9 -x "$SERVICE_NAME" 2>/dev/null || true
        sleep 1
    fi
    info "服务已停止"
}

do_start() {
    info "启动 ${SERVICE_NAME} 服务..."

    # 策略 1: systemctl
    if systemctl start "$SERVICE_NAME" 2>/dev/null; then
        info "  systemctl start 成功"
        return 0
    fi

    # 策略 2: start.sh 脚本
    if [ -x "${DEPLOY_DIR}/bin/start.sh" ]; then
        if bash "${DEPLOY_DIR}/bin/start.sh" 2>/dev/null; then
            info "  start.sh 启动成功"
            return 0
        fi
    fi

    # 策略 3: 直接启动二进制
    warn "  回退到直接启动二进制..."
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

# ─── 健康检查 ──────────────────────────────────────────────────────────────
health_check() {
    info "健康检查..."
    local retries=5
    local ok=false

    for i in $(seq 1 $retries); do
        local http_code
        http_code=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL" 2>/dev/null || echo "000")
        if [ "$http_code" = "200" ]; then
            ok=true
            break
        fi
        [ "$i" -lt "$retries" ] && sleep 3
    done

    if [ "$ok" = true ]; then
        info "  ✅ API: HTTP 200 OK"
        # 获取运行版本
        local status_json
        status_json=$(curl -s "$API_URL" 2>/dev/null || echo "{}")
        local running_ver=$(echo "$status_json" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
        [ -n "$running_ver" ] && info "  ✅ 运行版本: $running_ver"
        # 实例数
        local instances
        instances=$(curl -s "http://localhost:${API_PORT}/api/v1/instances" 2>/dev/null || echo "[]")
        local inst_count=$(echo "$instances" | grep -o '"id"' | wc -l)
        info "  ✅ 实例数: $inst_count"
        return 0
    else
        warn "  ⚠ API 未返回 200（可能需要更多启动时间）"
        return 1
    fi
}

# ─── 部署主流程 ────────────────────────────────────────────────────────────
do_deploy() {
    local pkg_path="$1"
    local pkg_name=$(basename "$pkg_path")
    local version=$(extract_version "$pkg_name")
    local TOTAL_STEPS=7

    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo "  GridSim 部署"
    echo "═══════════════════════════════════════════════════════════════"
    echo "  版本:   v${version}"
    echo "  包文件: ${pkg_name}"
    echo "  大小:   $(fmt_size $(stat -c%s "$pkg_path"))"
    echo "  部署到: ${DEPLOY_DIR}"
    echo "═══════════════════════════════════════════════════════════════"
    echo ""

    # 前置检查
    step 1 "前置检查"
    if [ ! -f "$pkg_path" ]; then
        err "包文件不存在: $pkg_path"
        exit 1
    fi
    # 验证 tar 包完整性
    local file_count
    file_count=$(tar tzf "$pkg_path" 2>/dev/null | wc -l)
    if [ "$file_count" -eq 0 ]; then
        err "包文件损坏或为空"
        exit 1
    fi
    info "  包内文件数: $file_count"

    # 检查磁盘空间
    local pkg_size=$(stat -c%s "$pkg_path")
    local needed=$((pkg_size * 2))
    mkdir -p "$DEPLOY_DIR"
    local avail=$(df --output=avail "$DEPLOY_DIR" | tail -1)
    avail=$((avail * 1024))
    if [ "$avail" -lt "$needed" ]; then
        err "磁盘空间不足: 可用 $(fmt_size $avail), 需要 $(fmt_size $needed)"
        exit 1
    fi
    info "  磁盘空间: $(fmt_size $avail) (充足)"

    # 判断是升级还是全新部署
    local is_upgrade=false
    if [ -f "${DEPLOY_DIR}/bin/start.sh" ] || [ -f "${DEPLOY_DIR}/bin/${SERVICE_NAME}" ]; then
        is_upgrade=true
        info "  模式: 🔄 升级"
    else
        info "  模式: 🆕 全新部署"
    fi

    # 停止服务
    step 2 "停止服务"
    if [ "$is_upgrade" = true ]; then
        do_stop
    else
        info "  (全新部署，无需停止)"
    fi

    # 备份配置
    step 3 "备份配置"
    if [ "$is_upgrade" = true ] && [ -d "${DEPLOY_DIR}/config" ] && [ "$(ls -A "${DEPLOY_DIR}/config" 2>/dev/null)" ]; then
        local ts=$(date '+%Y%m%d-%H%M%S')
        local backup_name="config-pre-upgrade-${ts}"
        local backup_path="${BACKUP_DIR}/${backup_name}"
        mkdir -p "$backup_path"
        cp -r "${DEPLOY_DIR}/config/"* "$backup_path/" 2>/dev/null || true
        # 写入 manifest
        echo "{\"backup_time\":\"$ts\",\"version\":\"$(cat "${DEPLOY_DIR}/bin/VERSION" 2>/dev/null || echo unknown)\"}" \
            > "$backup_path/manifest.json"
        info "  配置已备份 → ${backup_name}"
    else
        info "  (无需备份)"
    fi

    # 清理旧文件
    step 4 "清理旧文件"
    if [ "$is_upgrade" = true ]; then
        for item in "${DEPLOY_DIR}"/*; do
            local basename_item=$(basename "$item")
            # 保留 backups 目录
            [ "$basename_item" = "backups" ] && continue
            rm -rf "$item"
        done
        info "  旧文件已清理（保留 backups）"
    else
        info "  (全新部署，跳过清理)"
    fi

    # 解压新包
    step 5 "解压部署"
    tar xzf "$pkg_path" -C "$DEPLOY_DIR" --strip-components=1
    # 修复权限
    chmod +x "${DEPLOY_DIR}/bin/"* 2>/dev/null || true
    mkdir -p "${DEPLOY_DIR}/config" "${DEPLOY_DIR}/logs"
    info "  解压完成"
    info "  部署版本: $(cat "${DEPLOY_DIR}/bin/VERSION" 2>/dev/null || echo "$version")"

    # 恢复配置
    step 6 "恢复配置"
    if [ "$is_upgrade" = true ] && [ -d "$BACKUP_DIR" ]; then
        local latest_backup=$(ls -d "${BACKUP_DIR}"/config-pre-upgrade-* 2>/dev/null | sort -r | head -1)
        if [ -n "$latest_backup" ] && [ -d "$latest_backup" ]; then
            cp -r "$latest_backup"/* "${DEPLOY_DIR}/config/" 2>/dev/null || true
            rm -f "${DEPLOY_DIR}/config/manifest.json"
            info "  配置已恢复 (from $(basename "$latest_backup"))"
        else
            info "  (无备份可恢复)"
        fi
    else
        info "  (全新部署，跳过恢复)"
    fi

    # 启动服务
    step 7 "启动服务并验证"
    do_start
    sleep 2
    health_check

    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    info "🎉 部署完成!"
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
    echo "  管理命令:"
    echo "    systemctl status  ${SERVICE_NAME}    # 查看状态"
    echo "    systemctl restart ${SERVICE_NAME}    # 重启"
    echo "    systemctl stop    ${SERVICE_NAME}    # 停止"
    echo "    tail -f ${DEPLOY_DIR}/logs/output.log  # 查看日志"
    echo ""
    echo "  Web UI: http://localhost:${API_PORT}"
    local host_ip=$(hostname -I 2>/dev/null | awk '{print $1}')
    [ -n "$host_ip" ] && echo "  局域网: http://${host_ip}:${API_PORT}"
    echo ""
}

# ─── 回滚 ──────────────────────────────────────────────────────────────────
do_rollback() {
    info "回滚配置..."
    if [ ! -d "$BACKUP_DIR" ]; then
        err "无备份目录"
        exit 1
    fi

    local latest_backup=$(ls -d "${BACKUP_DIR}"/config-pre-upgrade-* 2>/dev/null | sort -r | head -1)
    if [ -z "$latest_backup" ]; then
        err "无可用备份"
        exit 1
    fi

    info "回滚到: $(basename "$latest_backup")"
    do_stop
    cp -r "$latest_backup"/* "${DEPLOY_DIR}/config/" 2>/dev/null || true
    rm -f "${DEPLOY_DIR}/config/manifest.json"
    do_start
    sleep 2
    health_check
    info "回滚完成"
}

# ─── 入口 ──────────────────────────────────────────────────────────────────
case "${1:-}" in
    --list|-l)
        list_versions
        ;;
    --status|-s)
        service_status
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
    --rollback)
        do_rollback
        ;;
    --version|-v)
        # 指定版本部署
        if [ -z "${2:-}" ]; then
            err "请指定版本号，例如: bash $0 --version 3.1.0-dev-14.5ea5bbe"
            echo "可用版本:"
            list_versions
            exit 1
        fi
        PKG=$(ls "${DIST_DIR}"/gridsim-v"$2"-linux-amd64.tar.gz 2>/dev/null | head -1)
        if [ -z "$PKG" ]; then
            err "未找到版本: $2"
            echo "提示: 文件名格式为 gridsim-v{version}-linux-amd64.tar.gz"
            list_versions
            exit 1
        fi
        do_deploy "$PKG"
        ;;
    --help|-h)
        echo "GridSim 本地部署脚本"
        echo ""
        echo "用法:"
        echo "  bash $0                         自动部署最新版本（从 DIST_DIR）"
        echo "  bash $0 <file.tar.gz>           直接指定包文件部署（新环境推荐）"
        echo "  bash $0 --version <ver>         部署指定版本（从 DIST_DIR）"
        echo "  bash $0 --list                  列出可用版本"
        echo "  bash $0 --status                查看服务状态"
        echo "  bash $0 --stop                  停止服务"
        echo "  bash $0 --start                 启动服务"
        echo "  bash $0 --restart               重启服务"
        echo "  bash $0 --rollback              回滚配置"
        echo ""
        echo "新环境快速部署:"
        echo "  1. 把 deploy-local.sh 和 gridsim-v*-linux-amd64.tar.gz 放到任意目录"
        echo "  2. 执行: bash deploy-local.sh ./gridsim-v3.1.0-linux-amd64.tar.gz"
        echo ""
        echo "环境变量覆盖:"
        echo "  GRIDSIM_DIST_DIR    构建产物目录 (当前: ${DIST_DIR})"
        echo "  GRIDSIM_DEPLOY_DIR  部署目标目录 (当前: ${DEPLOY_DIR})"
        echo "  GRIDSIM_PORT        API 端口     (当前: ${API_PORT})"
        echo ""
        echo "当前路径:"
        echo "  构建产物: ${DIST_DIR}"
        echo "  部署目录: ${DEPLOY_DIR}"
        echo "  服务名称: ${SERVICE_NAME}"
        echo "  API 端口: ${API_PORT}"
        ;;
    "")
        # 默认：自动选择最新版本部署
        PKG=$(get_latest_pkg)
        if [ -z "$PKG" ]; then
            err "未找到构建产物"
            echo "请先运行 build.sh 构建:"
            echo "  cd /root/IEC-SIM/iec104-sim-master && bash build.sh"
            exit 1
        fi
        do_deploy "$PKG"
        ;;
    *)
        # 尝试当作文件路径（支持 .tar.gz 直接部署）
        if [[ "$1" == *.tar.gz ]] && [ -f "$1" ]; then
            do_deploy "$1"
        else
            # 再尝试当作版本号
            PKG=$(ls "${DIST_DIR}"/gridsim-v"$1"-linux-amd64.tar.gz 2>/dev/null | head -1)
            if [ -n "$PKG" ]; then
                do_deploy "$PKG"
            else
                err "未知参数: $1"
                echo "如果是包文件路径，请确认文件存在；如果是版本号，请用 --list 查看可用版本"
                echo "使用 --help 查看帮助"
                exit 1
            fi
        fi
        ;;
esac
