#!/bin/bash
# ============================================================
# GridSim 远程构建脚本
#
# 用法:
#   bash _remote_build.sh                     # 仅构建（无变更时）
#   bash _remote_build.sh "commit message"    # 提交并构建
#
# 前置条件:
#   - Windows: 安装 sshpass (可通过 scoop install sshpass 或 cygwin)
#   - 或手动使用 SSH MCP 工具
#
# 输出:
#   远程服务器 dist/ 目录生成两个 .sh 安装包
# ============================================================
set -e

HOST="10.230.169.128"
USER="egc"
PASS="smart@123"
REMOTE_DIR="/home/egc/YH/GridSim-dev/gridsim"
BRANCH="develop"

echo "========================================"
echo "  GridSim 远程构建脚本"
echo "========================================"

# 检查是否有 sshpass
if ! command -v sshpass &> /dev/null; then
    echo "[WARN] sshpass 未安装，请使用 SSH MCP 工具或手动 SSH"
    echo "       Windows 用户可通过 scoop install sshpass 安装"
    exit 1
fi

# 1. 本地提交推送
if [ -n "$(git status --porcelain)" ]; then
    MSG="${1:-feat: update}"
    echo ""
    echo "[1/3] 本地提交: $MSG"
    git add -A
    git commit -m "$MSG"
    git push origin $BRANCH
else
    echo "[1/3] 无本地变更，跳过提交"
fi

# 2. 远程构建
echo ""
echo "[2/3] 远程构建..."
sshpass -p "$PASS" ssh -o StrictHostKeyChecking=no $USER@$HOST << 'REMOTE_SCRIPT'
set -e
export PATH=/usr/local/go/bin:$PATH
cd /home/egc/YH/GridSim-dev/gridsim

echo "  → 同步代码"
git fetch origin
git reset --hard origin/develop 2>/dev/null || {
    # 处理权限问题
    echo 'smart@123' | sudo -S chown -R egc:egc .git 2>/dev/null
    git reset --hard origin/develop
}

echo "  → 构建后端 (amd64 + arm64)"
make build-linux-amd64 build-linux-arm64

echo "  → 构建前端"
cd web
npm run build 2>&1 | tail -5
cd ..

echo "  → 生成 .sh 安装包"
bash make-installer.sh --all 2>&1 | tail -10

echo ""
echo "  → 产物清单"
ls -lh dist/gridsim-install-*.sh 2>/dev/null || echo "  [ERR] 未找到安装包"
REMOTE_SCRIPT

echo ""
echo "[3/3] 完成！"
echo ""
echo "安装包位置: $USER@$HOST:$REMOTE_DIR/dist/"
echo ""
echo "下载命令示例:"
echo "  scp $USER@$HOST:$REMOTE_DIR/dist/gridsim-install-*.sh ."
