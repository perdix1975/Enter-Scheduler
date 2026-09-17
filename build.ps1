$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

New-Item -ItemType Directory -Force -Path dist | Out-Null

# Embed the custom icon when the recovered .ico asset is present.
if (Test-Path 'assets\enter_scheduler.ico') {
    if (-not (Get-Command rsrc -ErrorAction SilentlyContinue)) {
        Write-Host 'Installing rsrc (one-time)...'
        go install github.com/akavel/rsrc@latest
        $env:Path += ";$env:USERPROFILE\go\bin"
    }

    if (Get-Command rsrc -ErrorAction SilentlyContinue) {
        rsrc -ico assets\enter_scheduler.ico -o rsrc_windows_amd64.syso
    }
}

go fmt ./...
go vet ./...
go build -trimpath -ldflags '-s -w -H=windowsgui' -o dist\Enter_Scheduler.exe .
Write-Host 'Built dist\Enter_Scheduler.exe'
