#!/usr/bin/env bash
# ============================================================
#  GridSim 一键发布脚本
#
#  功能：构建并输出双架构自解压安装包
#
#  用法：
#    bash release.sh              完整构建
#    bash release.sh --fast       快速构建（跳过 vue-tsc 类型检查）
#    bash release.sh --skip-web   仅后端构建
#
#  最终 Linux 产物（dist/）：
#    dist/gridsim-install-v{version}-linux-amd64.sh
#    dist/gridsim-install-v{version}-linux-arm64.sh
#
#  Linux .tar.gz 保留在 dist/，作为自解压安装包的嵌入载荷和可追溯构建产物。
#
#  使用方式：
#    scp dist/gridsim-install-*-linux-amd64.sh user@server:/tmp/
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

# ─── Step 1: 构建并生成安装包 ─────────────────────────────────────────────
info "[1/1] 执行构建并生成双架构安装包..."
echo ""
bash "$ROOT/build.sh" $BUILD_ARGS

# ─── 完成 ─────────────────────────────────────────────────────────────────
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
echo "═══════════════════════════════════════════════════════════════"
info "🎉 发布完成! (${ELAPSED}s)"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "  发布产物:"
ls -lh "$ROOT"/dist/gridsim-install-*.sh 2>/dev/null | awk '{printf "    %s  %s\n", $5, $9}'
echo ""
echo "  部署方式:"
echo "    scp dist/gridsim-install-*-linux-amd64.sh user@server:/tmp/"
echo "    ssh user@server \"bash /tmp/gridsim-install-*-linux-amd64.sh\""
echo ""
echo "  ARM64 部署:"
echo "    scp dist/gridsim-install-*-linux-arm64.sh user@arm-server:/tmp/"
echo "    ssh user@arm-server \"bash /tmp/gridsim-install-*-linux-arm64.sh\""
echo ""
