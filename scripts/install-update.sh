#!/bin/sh
# ============================================================================
# GridSim 一键更新安装脚本 (POSIX sh 兼容，适用于 sh/dash/busybox sh)
#
# 版本: 3.2.1-dev (2026-08-04)
# 用途: 将发行包 gridsim-v*.tar.gz 更新到现有 GridSim 安装目录。
#
# 特性:
#   1. 自动探测安装目录（运行中进程 / 常见路径 / PATH）
#   2. 更新前完整备份：config 配置 + 旧二进制 + VERSION（tar 归档 + 明文副本）
#   3. 只替换程序文件 (bin/、web/dist/、GUIDE.md)，绝不覆盖现有 config/ 与 logs/
#   4. 部署后自动验证：服务存活 + 版本号 + 实例 configured/running 数量逐一核对
#   5. 验证失败自动回滚到备份版本
#   6. 支持 --dry-run 预演，不产生任何改动
#
# 用法:
#   sh install-update.sh [安装目录] [发行包路径]
#   sh install-update.sh --help
#
# 选项:
#   -y, --yes          跳过安装目录确认（自动化场景）
#   -f, --force        当前版本与目标版本相同也强制更新
#   -k, --keep-on-fail 验证失败时不自动回滚，保留现场供排查
#   -n, --dry-run      预演：只打印将要执行的操作，不做任何修改
#
# 示例:
#   sh install-update.sh                          # 自动探测目录，使用同目录最新发行包
#   sh install-update.sh /opt/gridsim             # 指定安装目录
#   sh install-update.sh /opt/gridsim ./gridsim-v3.2.1-dev-1.561e902-linux-amd64.tar.gz
# ============================================================================

set -u

# ---------- 全局配置 ----------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HTTP_PORT="${GRIDSIM_HTTP_PORT:-8989}"
API_BASE="http://127.0.0.1:${HTTP_PORT}"
TIMEOUT_CURL=10
MAX_WAIT_START=15          # 启动最大等待秒数
MAX_WAIT_VERIFY=30         # 验证最大等待秒数

ASSUME_YES=0
FORCE=0
KEEP_ON_FAIL=0
DRY_RUN=0

# ---------- 工具函数 ----------
info()  { echo "[INFO ] $*"; }
warn()  { echo "[WARN ] $*"; }
ok()    { echo "[ OK  ] $*"; }
err()   { echo "[ERROR] $*"; }

die() {
    err "$*"
    exit 1
}

require_cmd() {
    for c in "$@"; do
        command -v "$c" >/dev/null 2>&1 || die "缺少必需命令: $c"
    done
}

# ---------- 参数解析 ----------
INSTALL_DIR=""
PACKAGE=""

while [ $# -gt 0 ]; do
    case "$1" in
        --help|-h)
            sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        --yes|-y) ASSUME_YES=1 ;;
        --force|-f) FORCE=1 ;;
        --keep-on-fail|-k) KEEP_ON_FAIL=1 ;;
        --dry-run|-n) DRY_RUN=1 ;;
        -*)
            die "未知选项: $1 (用 --help 查看用法)"
            ;;
        *)
            if [ -z "$INSTALL_DIR" ]; then
                INSTALL_DIR="$1"
            elif [ -z "$PACKAGE" ]; then
                PACKAGE="$1"
            else
                die "参数过多: $1"
            fi
            ;;
    esac
    shift
done

if [ -n "$DRY_RUN" ] && [ "$DRY_RUN" = "1" ]; then
    info "预演模式 (--dry-run)：只打印操作计划，不做任何修改"
fi

# ---------- 1. 定位发行包 ----------
if [ -z "$PACKAGE" ]; then
    # 同目录下找最新的 gridsim-v*.tar.gz
    PACKAGE=$(ls -t "$SCRIPT_DIR"/gridsim-v*.tar.gz 2>/dev/null | head -1)
    [ -z "$PACKAGE" ] && die "未找到发行包 gridsim-v*.tar.gz（请放到脚本同目录或通过参数指定）"
fi
[ -f "$PACKAGE" ] || die "发行包不存在: $PACKAGE"

NEW_VER=""
tar tzf "$PACKAGE" >/dev/null 2>&1 || die "发行包无法读取(可能损坏): $PACKAGE"
# 包内顶层目录名，如 gridsim-v3.2.1-dev-1.561e902-linux-amd64
PKG_TOP=$(tar tzf "$PACKAGE" 2>/dev/null | sed 's#^\./##; s#/.*##' | sort -u | head -1)
[ -z "$PKG_TOP" ] && die "发行包内目录结构异常: $PACKAGE"
# 读取包内版本号
NEW_VER=$(tar xzf "$PACKAGE" -O "$PKG_TOP/bin/VERSION" 2>/dev/null | tr -d ' \n')
[ -z "$NEW_VER" ] && NEW_VER="$PKG_TOP"
info "发行包: $PACKAGE"
info "目标版本: $NEW_VER"

# ---------- 2. 定位安装目录 ----------
detect_install_dir() {
    # a) 运行中的 gridsim serve 进程
    PIDS=$(ps -eo pid,args 2>/dev/null | grep -E '[g]ridsim.*serve' | awk '{print $1}')
    for p in $PIDS; do
        [ -r "/proc/$p/cmdline" ] || continue
        CFGDIR=$(tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null | grep -oE '\-\-config-dir[ =][^ ]+' | awk '{print $2}' | sed 's/^=//')
        if [ -n "$CFGDIR" ]; then
            D=$(cd "$CFGDIR/.." 2>/dev/null && pwd)
            [ -n "$D" ] && [ -x "$D/bin/gridsim" ] && { echo "$D"; return 0; }
        fi
        CWD=$(readlink "/proc/$p/cwd" 2>/dev/null)
        if [ -n "$CWD" ] && [ -x "$CWD/bin/gridsim" ]; then
            echo "$CWD"
            return 0
        fi
    done
    # b) 当前工作目录（用户通常在安装目录内执行脚本）
    if [ -x "$(pwd)/bin/gridsim" ]; then
        echo "$(pwd)"
        return 0
    fi
    # c) 常见部署路径
    for d in /opt/gridsim /root/gridsim /app/gridsim /usr/local/gridsim /srv/gridsim /data/gridsim /home/*/gridsim; do
        if [ -x "$d/bin/gridsim" ]; then echo "$d"; return 0; fi
    done
    # d) PATH 中的 gridsim
    G=$(command -v gridsim 2>/dev/null)
    if [ -n "$G" ]; then
        D=$(dirname "$(dirname "$G")")
        echo "$D"
        return 0
    fi
    return 1
}

if [ -z "$INSTALL_DIR" ]; then
    INSTALL_DIR=$(detect_install_dir) || die "无法自动定位安装目录，请用参数指定: sh install-update.sh /路径/到/安装目录"
fi
INSTALL_DIR=$(cd "$INSTALL_DIR" 2>/dev/null && pwd) || die "安装目录不存在: $INSTALL_DIR"
[ -x "$INSTALL_DIR/bin/gridsim" ] || die "安装目录中没有找到 bin/gridsim（目录不对？）: $INSTALL_DIR"

info "安装目录: $INSTALL_DIR"
if [ "$ASSUME_YES" != "1" ]; then
    printf "[CONF ] 确认更新安装目录为 %s ? [y/N] " "$INSTALL_DIR"
    read -r CONFIRM
    case "$CONFIRM" in
        y|Y|yes|YES) ;;
        *) die "已取消" ;;
    esac
fi

# ---------- 3. 版本比对 ----------
CUR_VER=""
if [ -f "$INSTALL_DIR/bin/VERSION" ]; then
    CUR_VER=$(tr -d ' \n' < "$INSTALL_DIR/bin/VERSION")
fi
info "当前版本: ${CUR_VER:-未知} → 目标版本: $NEW_VER"
if [ "$FORCE" != "1" ] && [ -n "$CUR_VER" ] && [ "$CUR_VER" = "$NEW_VER" ]; then
    die "当前版本已是最新 ($CUR_VER)，无需更新（如确需重装请加 --force）"
fi

# ---------- 3.5 记录部署前基线（用于更新后对比验证） ----------
BEFORE_CFG=0
BEFORE_RUN=0
record_before() {
    STATUS=$(curl -s --max-time "$TIMEOUT_CURL" "$API_BASE/api/v1/status")
    [ -z "$STATUS" ] && { warn "部署前状态 API 无响应，将以 configured/running 绝对值验证"; return; }
    BEFORE_CFG=$(echo "$STATUS" | grep -o '"configured":[0-9]*' | head -1 | cut -d: -f2)
    BEFORE_RUN=$(echo "$STATUS" | grep -o '"running":[0-9]*' | head -1 | cut -d: -f2)
    [ -z "$BEFORE_CFG" ] && BEFORE_CFG=0
    [ -z "$BEFORE_RUN" ] && BEFORE_RUN=0
    info "部署前基线: configured=$BEFORE_CFG running=$BEFORE_RUN"
}
if [ "$DRY_RUN" != "1" ]; then
    record_before
fi

# ---------- 4. 备份 ----------
BACKUP_ROOT="$INSTALL_DIR/backups"
BACKUP_DIR="$BACKUP_ROOT/gridsim-backup-$(date +%Y%m%d-%H%M%S)"
if [ "$DRY_RUN" = "1" ]; then
    info "[预演] 将创建备份目录: $BACKUP_DIR"
else
    mkdir -p "$BACKUP_DIR"
    # 4.1 明文副本：config 配置（实例、用户、点表、CSV、代理存储）
    cp -a "$INSTALL_DIR/config" "$BACKUP_DIR/config" 2>/dev/null || warn "config 目录备份失败（跳过）"
    cp -a "$INSTALL_DIR/bin/VERSION" "$BACKUP_DIR/OLD_VERSION" 2>/dev/null || true
    # 4.2 tar 归档：程序文件 + 配置（排除 logs/backups，避免日志与历史备份膨胀）
    tar czf "$BACKUP_DIR/rollback.tar.gz" \
        --exclude="backups" \
        --exclude="logs" \
        -C "$INSTALL_DIR" bin config GUIDE.md web 2>/dev/null \
        && ok "配置与旧版本已备份: $BACKUP_DIR/rollback.tar.gz" \
        || warn "归档备份部分失败（web/GUIDE 缺失时属正常）"
    # 4.3 记录备份清单
    {
        echo "备份时间: $(date '+%Y-%m-%d %H:%M:%S')"
        echo "安装目录: $INSTALL_DIR"
        echo "旧版本:   ${CUR_VER:-未知}"
        echo "新版本:   $NEW_VER"
        echo "备份内容: config/ 明文副本 + bin/config/web 归档"
    } > "$BACKUP_DIR/BACKUP_INFO.txt"
    info "备份完成: $BACKUP_DIR"
fi

# ---------- 5. 解包新版本到临时目录 ----------
TMP_DIR=$(mktemp -d 2>/dev/null || echo "$BACKUP_DIR/tmp")
if [ "$DRY_RUN" = "1" ]; then
    info "[预演] 将解包 $PACKAGE 到临时目录"
else
    tar xzf "$PACKAGE" -C "$TMP_DIR" || die "解包发行包失败"
fi
PKG_DIR="$TMP_DIR/$PKG_TOP"
[ -d "$PKG_DIR" ] || PKG_DIR=$(find "$TMP_DIR" -maxdepth 2 -type d -name "bin" | head -1 | sed 's#/bin$##')
[ "$DRY_RUN" = "1" ] || [ -x "$PKG_DIR/bin/gridsim" ] || die "解包后未找到 bin/gridsim（发行包结构异常）"

# 记录本次操作计划
info "更新计划:"
info "  停止 → 替换 bin/gridsim, bin/gridsim-mcp, bin/*.sh, bin/VERSION, web/dist/ → 启动 → 验证"
info "  保留: config/ 与 logs/ 目录（不覆盖现有配置）"

if [ "$DRY_RUN" = "1" ]; then
    info "[预演] 全部就绪，未做任何修改。可去掉 --dry-run 正式执行。"
    rm -rf "$TMP_DIR"
    exit 0
fi

# ---------- 6. 停止服务 ----------
STOPPED=0
if [ -x "$INSTALL_DIR/bin/stop.sh" ]; then
    if "$INSTALL_DIR/bin/stop.sh"; then
        STOPPED=1
    else
        warn "stop.sh 执行异常，尝试直接终止进程"
    fi
fi
# 兜底：直接终止残留进程
if [ "$STOPPED" != "1" ]; then
    pkill -f "gridsim.*serve" 2>/dev/null && { sleep 2; STOPPED=1; } || true
    pkill -f "gridsim serve" 2>/dev/null || true
fi
sleep 1

# ---------- 7. 替换文件 ----------
cp "$PKG_DIR/bin/gridsim"      "$INSTALL_DIR/bin/gridsim"
cp "$PKG_DIR/bin/gridsim-mcp"  "$INSTALL_DIR/bin/gridsim-mcp"
cp "$PKG_DIR/bin/VERSION"      "$INSTALL_DIR/bin/VERSION"
chmod +x "$INSTALL_DIR/bin/gridsim" "$INSTALL_DIR/bin/gridsim-mcp"
# 启停脚本随新版本更新（保持与发行包一致）
for s in start.sh stop.sh restart.sh; do
    if [ -f "$PKG_DIR/bin/$s" ]; then
        cp "$PKG_DIR/bin/$s" "$INSTALL_DIR/bin/$s"
        chmod +x "$INSTALL_DIR/bin/$s"
    fi
done
# 前端静态资源（先清空旧目录，避免 cp -r 嵌套成 web/dist/dist）
if [ -d "$PKG_DIR/web/dist" ]; then
    mkdir -p "$INSTALL_DIR/web"
    rm -rf "$INSTALL_DIR/web/dist"
    cp -r "$PKG_DIR/web/dist" "$INSTALL_DIR/web/dist"
fi
# 部署指南（不覆盖已有手册）
[ -f "$PKG_DIR/GUIDE.md" ] && [ ! -f "$INSTALL_DIR/GUIDE.md" ] && cp "$PKG_DIR/GUIDE.md" "$INSTALL_DIR/GUIDE.md"
sync
ok "程序文件已替换（config/ 与 logs/ 未动）"

# ---------- 8. 启动服务 ----------
if [ -x "$INSTALL_DIR/bin/start.sh" ]; then
    "$INSTALL_DIR/bin/start.sh" || warn "start.sh 启动失败，尝试直接启动"
fi
# 等待进程与端口就绪
STARTED=0
i=0
while [ "$i" -lt "$MAX_WAIT_START" ]; do
    if pgrep -f "gridsim.*serve" >/dev/null 2>&1; then
        STARTED=1
        break
    fi
    i=$((i + 1))
    sleep 1
done
[ "$STARTED" = "1" ] || die "服务进程未启动，回滚方案见文末（备份位于 $BACKUP_DIR）"

# ---------- 9. 验证 ----------
verify_instances() {
    # 参数: $1 = 期望版本号（回滚后校验旧版本）；空则跳过版本比对
    EXPECT_VER="${1:-}"
    STATUS=$(curl -s --max-time "$TIMEOUT_CURL" "$API_BASE/api/v1/status")
    [ -z "$STATUS" ] && { err "状态 API 无响应"; return 1; }
    CONFIGURED=$(echo "$STATUS" | grep -o '"configured":[0-9]*' | head -1 | cut -d: -f2)
    RUNNING=$(echo "$STATUS" | grep -o '"running":[0-9]*' | head -1 | cut -d: -f2)
    VER=$(echo "$STATUS" | grep -o '"version":"[^"]*"' | head -1 | cut -d'"' -f4)
    [ -z "$CONFIGURED" ] && CONFIGURED=0
    [ -z "$RUNNING" ] && RUNNING=0
    info "版本: $VER | configured: $CONFIGURED | running: $RUNNING (基线: $BEFORE_CFG/$BEFORE_RUN)"
    FAILED=0
    if [ -n "$EXPECT_VER" ]; then
        [ "$VER" != "$EXPECT_VER" ] && { err "版本号不符: 期望 $EXPECT_VER，实际 $VER"; FAILED=1; }
    fi
    [ "$CONFIGURED" != "$BEFORE_CFG" ] && { err "实例配置数变化: 期望 $BEFORE_CFG，实际 $CONFIGURED"; FAILED=1; }
    [ "$RUNNING" -lt "$BEFORE_RUN" ] && { err "运行实例减少: 更新前 $BEFORE_RUN，更新后 $RUNNING"; FAILED=1; }
    # 部署前全部实例在跑时，更新后必须全部在跑（running == configured），否则判定失败
    if [ "$BEFORE_RUN" -eq "$BEFORE_CFG" ] && [ "$BEFORE_RUN" -gt 0 ]; then
        [ "$RUNNING" -ne "$CONFIGURED" ] && { err "实例未全部启动: configured=$CONFIGURED running=$RUNNING"; FAILED=1; }
    fi
    [ "$FAILED" = "1" ] && return 1
    return 0
}

VERIFY_OK=0
i=0
while [ "$i" -lt "$MAX_WAIT_VERIFY" ]; do
    if verify_instances "$NEW_VER"; then
        VERIFY_OK=1
        break
    fi
    i=$((i + 3))
    sleep 3
done

if [ "$VERIFY_OK" = "1" ]; then
    echo ""
    ok "更新成功！"
    info "新版本:   $NEW_VER"
    info "安装目录: $INSTALL_DIR"
    info "备份位置: $BACKUP_DIR"
    info "Web UI:   $API_BASE"
    # 实例清单
    INSTS=$(curl -s --max-time "$TIMEOUT_CURL" "$API_BASE/api/v1/instances")
    N=$(echo "$INSTS" | grep -o '"status":"running"' | wc -l)
    ok "实例运行确认: $N 个实例全部 running"
    rm -rf "$TMP_DIR"
    exit 0
fi

# ---------- 10. 验证失败：回滚 ----------
err "验证失败"
if [ "$KEEP_ON_FAIL" = "1" ]; then
    err "已按 --keep-on-fail 保留现场，未回滚。"
    err "现场备份: $BACKUP_DIR (rollback.tar.gz)"
    exit 1
fi
warn "开始自动回滚到备份版本..."
"$INSTALL_DIR/bin/stop.sh" >/dev/null 2>&1 || pkill -f "gridsim.*serve" 2>/dev/null || true
sleep 1
tar xzf "$BACKUP_DIR/rollback.tar.gz" -C "$INSTALL_DIR" 2>/dev/null \
    || { err "回滚文件恢复失败！请手动从 $BACKUP_DIR 恢复"; exit 1; }
# 备份明文 config 副本恢复（tar 归档已含 config，此处兜底覆盖最新配置）
if [ -d "$BACKUP_DIR/config" ]; then
    rm -rf "$INSTALL_DIR/config"
    cp -a "$BACKUP_DIR/config" "$INSTALL_DIR/config"
fi
"$INSTALL_DIR/bin/start.sh" >/dev/null 2>&1 || true
sleep 3
if verify_instances "$CUR_VER"; then
    warn "已回滚到旧版本并恢复运行 (备份: $BACKUP_DIR)"
    exit 1
else
    err "回滚后服务仍异常！请手动处理，备份位于: $BACKUP_DIR"
    exit 1
fi
