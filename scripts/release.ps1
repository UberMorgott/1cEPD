#Requires -Version 7
<#
.SYNOPSIS
Собирает релиз Пульта ЭПД и, с -Publish, публикует его на GitHub.

.DESCRIPTION
Сборка — build-dist.ps1 с версией: exe с вшитой версией и архив пакета.
В релиз уходят два файла:
  pult-epd.exe      — его качает самообновление (selfupdate.AssetName);
  pult-epd-<ver>.zip — пакет для первой установки (exe, .env, start.bat, README).
Самообновление сверяет загрузку с sha256 «digest», который GitHub считает
сам при загрузке файла, поэтому файл контрольных сумм не публикуется.

.EXAMPLE
./scripts/release.ps1 -Version 0.1.0
./scripts/release.ps1 -Version 0.1.0 -Publish
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$Version,
    [switch]$Publish,
    # Собрать без UPX, если упакованный exe ловит антивирус.
    [switch]$NoUpx
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$Version = $Version.TrimStart('v')
if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "версия $Version не X.Y.Z" }
$repoSlug = 'UberMorgott/1cEPD' # internal/selfupdate.Repo
$root = Split-Path $PSScriptRoot -Parent
$out = Join-Path $root 'dist'
$tag = "v$Version"

# Публикуемый релиз собирается только из коммита своего тега: чистое дерево
# (неотслеживаемые файлы не в счёт), HEAD — это тег.
if ($Publish) {
    $changed = git -C $root status --porcelain --untracked-files=no
    if ($LASTEXITCODE) { throw "git status не прошёл в $root" }
    if ($changed) { throw "в рабочем дереве $root есть изменения; релиз собирается только из тега:`n$($changed -join "`n")" }
    $tagged = git -C $root rev-parse --verify --quiet "$tag^{commit}"
    if (-not $tagged) { throw "тега $tag нет: сначала поставьте тег на коммит релиза" }
    $head = git -C $root rev-parse HEAD
    if ($head -ne $tagged) { throw "HEAD $head — не $tag ($tagged): переключитесь на коммит тега" }
}

$buildArgs = @{ OutDir = $out; Version = $Version }
if ($NoUpx) { $buildArgs.NoUpx = $true }
& (Join-Path $root 'build-dist.ps1') @buildArgs
if (-not $?) { throw 'build-dist.ps1 не прошёл' }

$exe = Join-Path $out 'pult-epd-test\pult-epd.exe'
$asset = Join-Path $out 'pult-epd.exe'
$zip = Join-Path $out "pult-epd-$Version.zip"
Copy-Item -LiteralPath $exe -Destination $asset -Force
Copy-Item -LiteralPath (Join-Path $out 'pult-epd-test.zip') -Destination $zip -Force
$assets = @($asset, $zip)
foreach ($a in $assets) {
    '{0}  {1:N0} байт  sha256 {2}' -f (Split-Path $a -Leaf), (Get-Item $a).Length, (Get-FileHash -Algorithm SHA256 $a).Hash.ToLower()
}

if ($Publish) {
    gh release view $tag --repo $repoSlug *> $null
    if ($LASTEXITCODE -eq 0) {
        gh release upload $tag @assets --clobber --repo $repoSlug
    } else {
        $prev = git -C $root describe --tags --abbrev=0 "$tag^" 2>$null
        $range = if ($prev) { "$prev..$tag" } else { $tag }
        $notes = Join-Path $out 'notes.md'
        $lines = git -C $root log --no-merges --format='- %s' $range
        @("## Пульт ЭПД $tag", '', $lines, '', 'Первая установка — архив `pult-epd-' + $Version + '.zip`; уже установленные копии обновятся сами.') |
            Set-Content -Encoding utf8NoBOM $notes
        gh release create $tag @assets --repo $repoSlug --title $tag --notes-file $notes --verify-tag
    }
    if ($LASTEXITCODE) { throw "gh release не прошёл для $tag" }
}
