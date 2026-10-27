# tests 目录便捷入口：转发到 server/scripts/test-regression.ps1
#
# 用法：
#   powershell -NoProfile -ExecutionPolicy Bypass -File server/tests/run.ps1
#   powershell -NoProfile -ExecutionPolicy Bypass -File server/tests/run.ps1 -Coverage
#
# 参数与主脚本一致：-Coverage / -Run / -Timeout

[CmdletBinding()]
param(
    [switch]$Coverage,
    [string]$Run = "./tests/...",
    [string]$Timeout = "15m"
)

$serverDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$mainScript = Join-Path $serverDir "scripts\test-regression.ps1"
if (-not (Test-Path -LiteralPath $mainScript)) {
    throw "未找到主脚本: $mainScript"
}

& $mainScript -Coverage:$Coverage -Run $Run -Timeout $Timeout
exit $LASTEXITCODE
