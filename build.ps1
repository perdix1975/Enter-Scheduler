$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

New-Item -ItemType Directory -Force -Path 'dist\assets' | Out-Null

$env:CGO_ENABLED = '0'
go fmt ./...
go vet ./...
go build -trimpath -ldflags '-s -w -H=windowsgui' -o dist\Enter_Scheduler.exe .
Copy-Item assets\enter_scheduler.ico dist\assets\enter_scheduler.ico -Force

Write-Host 'Built dist\Enter_Scheduler.exe and copied dist\assets\enter_scheduler.ico'
