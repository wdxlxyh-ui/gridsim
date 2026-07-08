#!/usr/bin/env bash
# ============================================================
#  GridSim 一键发布脚本
#
#  功能：构建 + 打包 + 生成自解压安装包（amd64 + arm64）
#
#  用法：
#    bash release.sh              完整构建 + 生成双架构安装包
#    bash release.sh --fast       快速构建（跳过 vue-tsc 类型检查）
#    bash release.sh --skip-web   仅后端构建 + 生成安装包
#
#  输出产物（在项目根目录）：
#    gridsim-install-v{version}-linux-amd64.sh
#    gridsim-install-v{version}-linux-arm64.sh
#
#  使用方式：
#    scp gridsim-install-*-linux-amd64.sh user@server:/tmp/
#    ssh user@server "bash /tmp/gridsim-install-*-linux-amd64.sh"
# ============================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $*"; }

# ─── 解析参数 ─────────────────────────────────────────────────────────────
BUILD_ARGS=""
for arg in "$@"; do
    case "$arg" in
        --fast)      BUILD_ARGS="--fast" ;;
        --skip-web)  BUILD_ARGS="--skip-web" ;;
        --help|-h)
            echo "GridSim 一键发布脚本"
            echo ""
            echo "用法:"
            echo "  bash $0              完整构建 + 双架构安装包"
            echo "  bash $0 --fast       快速构建（跳过类型检查）"
            echo "  bash $0 --skip-web   仅后端 + 双架构安装包"
            echo ""
            exit 0
            ;;
        *)
            echo "未知参数: $arg"
            exit 1
            ;;
    esac
done

START_TIME=$(date +%s)

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  GridSim 一键发布"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# ─── Step 1: 构建 ─────────────────────────────────────────────────────────
info "[1/2] 执行构建..."
echo ""
bash "$ROOT/build.sh" $BUILD_ARGS
echo ""

# ─── Step 2: 生成安装包 ───────────────────────────────────────────────────
info "[2/2] 生成自解压安装包 (amd64 + arm64)..."
echo ""
bash "$ROOT/make-installer.sh" --all

# ─── 完成 ─────────────────────────────────────────────────────────────────
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
echo "═══════════════════════════════════════════════════════════════"
info "🎉 发布完成! (${ELAPSED}s)"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "  发布产物:"
ls -lh "$ROOT"/gridsim-install-*.sh 2>/dev/null | awk '{printf "    %s  %s\n", $5, $9}'
echo ""
echo "  部署方式:"
echo "    scp gridsim-install-*-linux-amd64.sh user@server:/tmp/"
echo "    ssh user@server \"bash /tmp/gridsim-install-*-linux-amd64.sh\""
echo ""
echo "  ARM64 部署:"
echo "    scp gridsim-install-*-linux-arm64.sh user@arm-server:/tmp/"
echo "    ssh user@arm-server \"bash /tmp/gridsim-install-*-linux-arm64.sh\""
echo ""
