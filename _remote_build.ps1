# ============================================================
# GridSim 远程构建脚本 (PowerShell 版)
#
# 用法:
#   .\_remote_build.ps1                     # 仅构建
#   .\_remote_build.ps1 -Message "msg"      # 提交并构建
#
# 依赖:
#   - plink (PuTTY 命令行工具) 或 OpenSSH
#   - 建议通过 Kiro 的 SSH MCP 工具执行
# ============================================================

param(
    [string]$Message = "",
    [switch]$SkipCommit
)

$Host = "10.230.169.128"
$User = "egc"
$Pass = "smart@123"
$RemoteDir = "/home/egc/YH/GridSim-dev/gridsim"
$Branch = "develop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  GridSim 远程构建脚本" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# 检查是否有变更
$status = git status --porcelain
if ($status -and -not $SkipCommit) {
    if (-not $Message) {
        $Message = Read-Host "请输入提交信息"
    }
    Write-Host "`n[1/3] 本地提交: $Message" -ForegroundColor Yellow
    git add -A
    git commit -m $Message
    git push origin $Branch
} else {
    Write-Host "[1/3] 无本地变更，跳过提交" -ForegroundColor Green
}

Write-Host "`n[2/3] 远程构建..." -ForegroundColor Yellow
Write-Host "请使用 Kiro SSH MCP 工具执行以下命令:`n" -ForegroundColor White

$remoteCommands = @"
export PATH=/usr/local/go/bin:\$PATH
cd $RemoteDir

echo '=== 同步代码 ==='
git fetch origin
git reset --hard origin/$Branch

echo '=== 构建后端 ==='
make build-linux-amd64 build-linux-arm64

echo '=== 构建前端 ==='
cd web && npm run build && cd ..

echo '=== 生成安装包 ==='
bash make-installer.sh --all

echo '=== 产物清单 ==='
ls -lh dist/gridsim-install-*.sh
"@

Write-Host $remoteCommands -ForegroundColor Gray

Write-Host "`n[3/3] 提示" -ForegroundColor Yellow
Write-Host "复制上述命令，在 Kiro 中执行 SSH MCP 工具完成构建" -ForegroundColor White
Write-Host "`n安装包位置: ${User}@${Host}:${RemoteDir}/dist/" -ForegroundColor Green
