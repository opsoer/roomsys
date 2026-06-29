Write-Host "=== 构建后端 ===" -ForegroundColor Cyan
Set-Location server
go build -o rental-server.exe .
if ($?) {
    Write-Host "后端构建成功" -ForegroundColor Green
} else {
    Write-Host "后端构建失败" -ForegroundColor Red
    exit 1
}

Write-Host "`n=== 构建前端 ===" -ForegroundColor Cyan
Set-Location ../web
npm run build
if ($?) {
    Write-Host "前端构建成功" -ForegroundColor Green
} else {
    Write-Host "前端构建失败" -ForegroundColor Red
    exit 1
}

Write-Host "`n=== 启动服务器 ===" -ForegroundColor Cyan
Set-Location ../server
./rental-server.exe
