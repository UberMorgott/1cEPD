#Requires -Version 7
<#
.SYNOPSIS
Собирает архив Пульта ЭПД для передачи коллегам.

.DESCRIPTION
Один exe со встроенными SPA и шаблоном формы заявки плюс раздаточные .env,
start.bat и README.txt.

Сжатие UPX включено по умолчанию: втрое меньше при той же работе. Оно же --
частая причина ложных срабатываний антивирусов, поэтому -NoUpx собирает то же
самое без упаковки, для тех, у кого файл уезжает в карантин.

.EXAMPLE
./build-dist.ps1
./build-dist.ps1 -NoUpx -OutDir C:\dist
#>
[CmdletBinding()]
param(
    # Куда сложить папку пакета и архив.
    [string]$OutDir = (Join-Path $PSScriptRoot 'dist'),
    # Собрать без UPX, если упакованный exe ловит антивирус.
    [switch]$NoUpx,
    # Пропустить проверку запуска (нужна свободная сеть на localhost).
    [switch]$SkipSmokeTest,
    # Порт для проверки запуска.
    [int]$SmokePort = 8199,
    # Версия X.Y.Z: по ней работает самообновление. Без неё сборка — «dev» и
    # сама не обновляется.
    [string]$Version = 'dev'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repo = $PSScriptRoot
$pkg = Join-Path $OutDir 'pult-epd-test'
$zip = Join-Path $OutDir 'pult-epd-test.zip'

# --- пакет собираем с нуля: остатки прошлого прогона в архив не попадают ---
if (Test-Path -LiteralPath $pkg) { Remove-Item -LiteralPath $pkg -Recurse -Force }
$null = New-Item -ItemType Directory -Path $pkg -Force

# --- фронт: dist лежит в репозитории, но пересобрать дешевле, чем гадать ---
Push-Location (Join-Path $repo 'web')
try {
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'npm run build не прошёл' }
} finally { Pop-Location }

# --- сервер: -s -w срезает символы и DWARF, -trimpath убирает пути машины.
# -H=windowsgui убирает консольное окно: программа живёт значком в трее, а
# ошибки старта показывает окном и пишет в pult-epd.log рядом с exe ---
$exe = Join-Path $pkg 'pult-epd.exe'
$Version = $Version.TrimStart('v')
if ($Version -ne 'dev' -and $Version -notmatch '^\d+\.\d+\.\d+$') { throw "версия $Version не X.Y.Z" }
$ldflags = "-s -w -H=windowsgui -X partnerops/internal/selfupdate.Version=$Version"
& go build -trimpath -ldflags $ldflags -o $exe (Join-Path $repo 'cmd\server')
if ($LASTEXITCODE -ne 0) { throw 'go build не прошёл' }
$rawSize = (Get-Item $exe).Length

if (-not $NoUpx) {
    $upx = (Get-Command upx -ErrorAction SilentlyContinue)?.Source
    if (-not $upx) { throw 'upx не найден в PATH. Поставить: winget install --id UPX.UPX, либо собрать с -NoUpx' }
    & $upx --best --lzma $exe
    if ($LASTEXITCODE -ne 0) { throw 'upx не смог упаковать' }
}

# env.template, а не .env: .gitignore прячет .env в любой папке, и шаблон
# раздатки уехал бы вместе с боевым файлом. В пакете он снова .env.
foreach ($item in @(
        @{ From = 'dist-template\env.template'; To = '.env' }
        @{ From = 'dist-template\start.bat'; To = 'start.bat' }
        @{ From = 'dist-template\README.txt'; To = 'README.txt' }
    )) {
    $src = Join-Path $repo $item.From
    if (-not (Test-Path -LiteralPath $src)) { throw "нет шаблона раздатки: $src" }
    Copy-Item -LiteralPath $src -Destination (Join-Path $pkg $item.To)
}

# --- проверка: пакет должен подниматься и пускать по логину из .env ---
# Сборка с -H=windowsgui окна не открывает, поэтому Start-Process тут ничем не
# отличается от прежнего: процесс живёт, пока его не снимут. Значок в трее на
# эти пять секунд появится -- это нормально. NO_BROWSER гасит автооткрытие
# браузера, иначе каждая сборка пакета распахивала бы вкладку.
if (-not $SkipSmokeTest) {
    $env:LISTEN_ADDR = "127.0.0.1:$SmokePort"
    $env:NO_BROWSER = '1'
    $proc = Start-Process -FilePath $exe -WorkingDirectory $pkg -PassThru -WindowStyle Hidden
    try {
        Start-Sleep -Seconds 5
        $base = "http://127.0.0.1:$SmokePort"
        if ((Invoke-WebRequest -Uri $base -UseBasicParsing -TimeoutSec 10).StatusCode -ne 200) { throw 'страница не отдалась' }
        $creds = Get-Content (Join-Path $pkg '.env') | Select-String -Pattern '^APP_(LOGIN|PASSWORD)='
        $login = ($creds | Where-Object { $_ -match 'APP_LOGIN=' }) -replace '.*APP_LOGIN=', ''
        $password = ($creds | Where-Object { $_ -match 'APP_PASSWORD=' }) -replace '.*APP_PASSWORD=', ''
        $body = @{ login = $login.Trim(); password = $password.Trim() } | ConvertTo-Json -Compress
        $login = $password = $null
        $answer = Invoke-WebRequest -Uri "$base/api/login" -Method POST -Body $body -ContentType 'application/json' -UseBasicParsing -TimeoutSec 10
        if ($answer.StatusCode -ne 200) { throw "вход не прошёл: $($answer.StatusCode)" }
        $health = Invoke-RestMethod -Uri "$base/api/health" -TimeoutSec 10
        if ($health.version -ne $Version) { throw "сборка отвечает версией '$($health.version)', ждали $Version" }
    } finally {
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 1
        $env:LISTEN_ADDR = $null
        $env:NO_BROWSER = $null
    }
}

# --- база и журнал от проверки коллегам не нужны: пусть начинают с чистого ---
foreach ($leftover in @('data', 'pult-epd.log')) {
    $path = Join-Path $pkg $leftover
    if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Recurse -Force }
}

if (Test-Path -LiteralPath $zip) { Remove-Item -LiteralPath $zip -Force }
Compress-Archive -Path (Join-Path $pkg '*') -DestinationPath $zip -CompressionLevel Optimal

$exeSize = (Get-Item $exe).Length
'{0}  exe {1:N0} -> {2:N0} байт  архив {3:N2} МБ' -f (git -C $repo rev-parse --short HEAD), $rawSize, $exeSize, ((Get-Item $zip).Length / 1MB)
$zip
