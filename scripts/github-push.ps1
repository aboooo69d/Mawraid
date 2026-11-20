<#
.SYNOPSIS
    Mawraid Backend - Fast GitHub Sync & Quality Gate
#>
[CmdletBinding()]
param (
    [string]$Type = "feat",
    [string]$Scope = "",
    [string]$Message = "",
    [switch]$SkipTests = $false
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $RepoRoot
try {
    Write-Host "`n🚀 Mawraid Backend GitHub Deployer" -ForegroundColor Cyan
    Write-Host "======================================" -ForegroundColor DarkCyan

    if (-not $Message) {
        $Message = Read-Host "أدخل رسالة الـ Commit"
        while (-not $Message.Trim()) {
            $Message = Read-Host "رسالة الـ Commit مطلوبة"
        }
    }

    $commitHeader = if ($Scope) { "$($Type.ToLower())($($Scope.ToLower())): $Message" } else { "$($Type.ToLower()): $Message" }

    # Run Tests
    if (-not $SkipTests) {
        Write-Host "⚡ تشغيل اختبارات الوحدة..." -ForegroundColor Gray
        $go = if (Get-Command go -ErrorAction SilentlyContinue) { "go" } else { "C:\Program Files\Go\bin\go.exe" }
        & $go test -count=1 ./tests/...
        if ($LASTEXITCODE -ne 0) {
            throw "فشلت اختبارات الوحدة! تم إيقاف الرفع للحفاظ على استقرار المشروع."
        }
        Write-Host "✅ كافة الاختبارات سليمة 100%!" -ForegroundColor Green
    }

    # Git sync
    $branch = (git branch --show-current).Trim()
    if (-not $branch) { $branch = "main" }

    git pull --rebase origin $branch 2>&1 | Out-Null
    
    $status = git status --porcelain
    if (-not $status) {
        Write-Host "✅ لا توجد أي تعديلات غير مرفوعة." -ForegroundColor Green
        return
    }

    git add -A
    git commit -m "$commitHeader"
    git push origin $branch

    Write-Host "`n🎉 تم الرفع بنجاح إلى GitHub! [$commitHeader]`n" -ForegroundColor Green
} finally {
    Pop-Location
}
