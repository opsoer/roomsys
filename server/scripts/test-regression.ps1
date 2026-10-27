# Go API 回归测试一键执行
#
# 用法（在仓库任意目录）：
#   powershell -NoProfile -ExecutionPolicy Bypass -File server/scripts/test-regression.ps1
#   powershell -NoProfile -ExecutionPolicy Bypass -File server/scripts/test-regression.ps1 -Coverage
#
# 说明：
#   - 测试使用进程内 httptest + 临时 SQLite，不需要启动 8081 后端。
#   - 默认串行执行（-p 1），因为时间偏移/限流/统计缓存是进程级全局状态。
#   - 若安装了 gotestsum，会额外生成 JUnit XML 到 server/tests/artifacts/。
#   - 未安装 gotestsum 时回退到 go test，功能不受影响。

[CmdletBinding()]
param(
    [switch]$Coverage,
    [string]$Run = "./tests/...",
    [string]$Timeout = "15m"
)

$ErrorActionPreference = "Stop"
$serverDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
# Go 回归产物统一放在 tests/artifacts 下，和仓库根的 AI/手工 test-evidence 分开
$evidenceDir = Join-Path $serverDir "tests\artifacts"
New-Item -ItemType Directory -Path $evidenceDir -Force | Out-Null

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$junit = Join-Path $evidenceDir "regression-$stamp.xml"
$coverageFile = Join-Path $evidenceDir "coverage-$stamp.out"

$testArgs = @($Run, "-count=1", "-p", "1", "-timeout", $Timeout)
if ($Coverage) {
    $testArgs += @("-coverprofile", $coverageFile, "-coverpkg=./handlers/...,./services/...,./middleware/...,./utils/...,./models/...")
}

Push-Location $serverDir
try {
    $gotestsum = Get-Command gotestsum -ErrorAction SilentlyContinue
    if (-not $gotestsum) {
        $gp = (& go env GOPATH).Trim()
        foreach ($name in @("gotestsum.exe", "gotestsum")) {
            $candidate = Join-Path $gp "bin\$name"
            if (Test-Path -LiteralPath $candidate) { $gotestsum = [pscustomobject]@{ Source = $candidate }; break }
        }
    }
    if ($gotestsum) {
        Write-Host "==> gotestsum $($gotestsum.Source)" -ForegroundColor Cyan
        $args = @("--format", "testname", "--junitfile", $junit, "--") + $testArgs
        & $gotestsum.Source @args
    }
    else {
        Write-Host "==> gotestsum 未安装，使用 go test" -ForegroundColor Yellow
        Write-Host "    安装后可生成 JUnit: go install gotest.tools/gotestsum@latest" -ForegroundColor Yellow
        & go test @testArgs
    }
    $code = $LASTEXITCODE

    if ($Coverage -and (Test-Path -LiteralPath $coverageFile)) {
        Write-Host "==> 覆盖率汇总" -ForegroundColor Cyan
        & go tool cover -func $coverageFile 2>$null | Select-Object -Last 1
        if ($LASTEXITCODE -ne 0) {
            Write-Host "    当前 Go 安装缺少 covdata，覆盖率汇总不可用；测试结果不受影响。" -ForegroundColor Yellow
        }
        Write-Host "    覆盖率文件: $coverageFile"
    }
    if (Test-Path -LiteralPath $junit) {
        Write-Host "    JUnit: $junit"
    }
    exit $code
}
finally {
    Pop-Location
}
