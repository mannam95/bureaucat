#Requires -Version 5.1
<#
.SYNOPSIS
  Windows-native port of the Makefile — no make, bash, openssl, curl, awk or sed needed.
  Only external requirements: Docker Desktop (compose v2), Python (for `seed`), git (for `release`).

.USAGE
  .\tasks.ps1                  # list all tasks
  .\tasks.ps1 bootstrap        # one-command setup (dev)
  .\tasks.ps1 dev-up
  .\tasks.ps1 dev-logs -S app  # pass -S for a single service
  .\tasks.ps1 prod-bootstrap
  .\tasks.ps1 release -Version 1.2.3 -AllowDirty
#>
param(
  [Parameter(Position = 0)] [string]$Task = "help",
  [string]$S = "",              # service name for dev-logs / prod-logs
  [string]$Version = "1.2.2",   # manual release version (CI is the normal path)
  [switch]$AllowDirty           # skip the dirty-tree check for release
)

$ErrorActionPreference = "Stop"

# ---- shared config (mirrors make/00-config.mk) ----------------------------
$AppName   = "app"
$PgService = "postgres"
$Bin       = "/app/tmp/main"
$AppUrl    = "http://localhost:1341"
$HealthUrl = "$AppUrl/api/v1/health"
$EnvFile   = ".env"
$EnvExample = ".env.example"
$GarageToml = "garage/garage.toml"
$GarageContainer = "bureaucat-garage"

$DockerUser = "mvsrinath"
$ImageName  = "sprintboard"
$Image      = "docker.io/$DockerUser/$ImageName"
$Platform   = "linux/amd64"

# ---- helpers --------------------------------------------------------------

# Run a native command, tolerating its stderr output. PowerShell 5.1 turns
# redirected native stderr (e.g. docker's harmless "No resource found to
# remove" warnings) into error records, which are fatal under
# $ErrorActionPreference = Stop. This wrapper downgrades the preference
# around the call, so 2>$null genuinely discards the noise instead of crashing.
function Invoke-NativeQuiet {
  param([string]$File, [string[]]$NativeArgs)
  $prev = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try { & $File @NativeArgs 2>$null } finally { $ErrorActionPreference = $prev }
}

# Run docker compose for the dev or prod stack. Extra args are passed through.
# -Quiet swallows stderr (for cleanup calls where "nothing to remove" is fine).
function Invoke-DC {
  param([bool]$Prod, [string[]]$DcArgs, [switch]$Quiet)
  $cmd = @("compose", "-p")
  if ($Prod) { $cmd += "bureaucat-prod"; $cmd += @("-f", "docker-compose.prod.yml") }
  else       { $cmd += "bureaucat" }
  $cmd += $DcArgs
  if ($Quiet) { Invoke-NativeQuiet docker $cmd }
  else        { & docker @cmd }
}

# 64 hex chars of randomness without openssl (two GUIDs = 64 hex chars).
function New-Hex64 { ([guid]::NewGuid().ToString("N") + [guid]::NewGuid().ToString("N")) }

# Poll a docker container until its healthcheck reports healthy.
function Wait-ContainerHealthy {
  param([string]$Container)
  Write-Host "-> waiting for $Container to become healthy..."
  while ($true) {
    $status = Invoke-NativeQuiet docker @("inspect", "-f", "{{.State.Health.Status}}", $Container)
    if ($status -eq "healthy") { break }
    Start-Sleep -Seconds 2
  }
}

# Poll an HTTP endpoint until it answers (no curl needed).
function Wait-Http {
  param([string]$Url)
  Write-Host "-> waiting for $Url to answer..."
  while ($true) {
    try {
      Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5 | Out-Null
      break
    } catch { Start-Sleep -Seconds 2 }
  }
}

# Find a working python (Windows usually has `python`, not `python3`).
function Get-Python {
  foreach ($py in @("python", "python3")) {
    if (Get-Command $py -ErrorAction SilentlyContinue) { return $py }
  }
  throw "python not found on PATH - needed for `seed` (https://python.org)"
}

# ---- setup pieces (mirrors make/20-setup.mk) ------------------------------

function New-EnvFile {
  if (-not (Test-Path $EnvFile)) {
    Copy-Item $EnvExample $EnvFile
    Write-Host "-> created $EnvFile"
  }
}

function New-GarageConfig {
  if (Test-Path $GarageToml) { return }
  New-Item -ItemType Directory -Force -Path "garage/meta", "garage/data" | Out-Null
  $rpc = New-Hex64; $adm = New-Hex64
  $toml = @"
metadata_dir = "/var/lib/garage/meta"
data_dir = "/var/lib/garage/data"
db_engine = "sqlite"

replication_factor = 1

rpc_bind_addr = "[::]:3901"
rpc_public_addr = "127.0.0.1:3901"
rpc_secret = "$rpc"

[s3_api]
s3_region = "garage"
api_bind_addr = "[::]:3900"
root_domain = ".s3.garage.localhost"

[s3_web]
bind_addr = "[::]:3902"
root_domain = ".web.garage.localhost"
index = "index.html"

[admin]
api_bind_addr = "[::]:3903"
admin_token = "$adm"
"@
  [System.IO.File]::WriteAllText("$PWD/$GarageToml", $toml)  # UTF-8, no BOM
  Write-Host "-> generated $GarageToml (fresh secrets)"
}

# PowerShell port of garage-init.sh: provision cluster layout + bucket + key
# via `docker exec`, and wire the credentials into .env.
function Invoke-GarageInit {
  $bucket = "bureaucat"
  $keyName = "bureaucat-bucket-key"

  function G { param([string[]]$GaArgs)
    Invoke-NativeQuiet docker (@("exec", $GarageContainer, "/garage") + $GaArgs)
  }

  Write-Host "Waiting for Garage ($GarageContainer)..."
  while ($true) {
    G @("status") | Out-Null
    if ($LASTEXITCODE -eq 0) { break }
    Start-Sleep -Seconds 2
  }

  # 1. Assign a cluster layout if this node has no role yet.
  $status = G @("status")
  $line = $status | Where-Object { $_ -match "NO ROLE ASSIGNED" } | Select-Object -First 1
  if ($line) {
    $nodeId = ($line.Trim() -split "\s+")[0]
    Write-Host "Assigning layout to node $nodeId..."
    G @("layout", "assign", $nodeId, "-z", "dc1", "-c", "1G") | Out-Null
    G @("layout", "apply", "--version", "1") | Out-Null
    while ($true) {   # bucket/key ops fail until the layout is live
      G @("bucket", "list") | Out-Null
      if ($LASTEXITCODE -eq 0) { break }
      Start-Sleep -Seconds 1
    }
    Write-Host "Layout applied."
  }

  # 2. Create the bucket if it doesn't exist.
  if (-not ((G @("bucket", "list")) -match "\b$bucket\b")) {
    Write-Host "Creating bucket '$bucket'..."
    G @("bucket", "create", $bucket) | Out-Null
  }

  # 3. Create the access key if missing (secret is only revealed at creation).
  $secretKey = ""
  $keyList = G @("key", "list")
  if ($keyList -match $keyName) {
    $accessKeyId = (($keyList | Where-Object { $_ -match $keyName } | Select-Object -First 1).Trim() -split "\s+")[0]
    Write-Host "Key '$keyName' already exists ($accessKeyId)."
  } else {
    Write-Host "Creating key '$keyName'..."
    $keyOut = (G @("key", "create", $keyName)) -join "`n"
    $accessKeyId = ([regex]::Match($keyOut, "Key ID:\s*(\S+)")).Groups[1].Value
    $secretKey   = ([regex]::Match($keyOut, "Secret key:\s*(\S+)")).Groups[1].Value
  }

  # 4. Grant the key read+write on the bucket (idempotent).
  G @("bucket", "allow", "--read", "--write", $bucket, "--key", $keyName) | Out-Null

  # 5. Wire the credentials into .env (only possible right after key creation).
  if ($secretKey -and (Test-Path $EnvFile)) {
    $lines = Get-Content $EnvFile
    $lines = $lines -replace "^FILES_BUCKET_ACCESS_KEY_ID=.*", "FILES_BUCKET_ACCESS_KEY_ID=$accessKeyId"
    $lines = $lines -replace "^FILES_BUCKET_SECRET_ACCESS_KEY=.*", "FILES_BUCKET_SECRET_ACCESS_KEY=$secretKey"
    [System.IO.File]::WriteAllLines("$PWD/$EnvFile", $lines)
    Write-Host "Wrote bucket credentials into $EnvFile"
  } elseif (-not $secretKey) {
    Write-Host "Key already existed; $EnvFile left unchanged (Garage reveals the secret only at creation)."
  }

  Write-Host "Garage init complete."
}

# Stop + remove BOTH stacks so dev/prod can't clash on ports or the shared
# bureaucat-garage container name.
function Stop-All {
  Write-Host "-> stopping any existing dev/prod stacks..."
  Invoke-DC -Prod $false -Quiet @("down", "--remove-orphans")
  Invoke-DC -Prod $true  -Quiet @("down", "--remove-orphans")
}

function Invoke-Bootstrap {
  param([bool]$Prod)
  New-EnvFile; New-GarageConfig; Stop-All

  $label = if ($Prod) { "PROD" } else { "DEV" }
  Write-Host "-> starting postgres + garage..."
  Invoke-DC -Prod $Prod @("up", "-d", "--build", $PgService, "garage")
  Wait-ContainerHealthy $GarageContainer

  Write-Host "-> creating S3 bucket + access key, wiring them into .env..."
  Invoke-GarageInit

  Write-Host "-> starting the app..."
  Invoke-DC -Prod $Prod @("up", "-d", "--build")
  Wait-Http $HealthUrl

  if ($Prod) {
    $env:PG_CONTAINER = "bureaucat-prod-postgres-1"
    try { & (Get-Python) tools/seed.py } finally { Remove-Item Env:PG_CONTAINER -ErrorAction SilentlyContinue }
    Write-Host ""
    Write-Host "Prod is up at $AppUrl  (demo login: demo@gmail.com / Passw0rd!)"
  } else {
    Invoke-DC -Prod $false @("exec", "-T", $AppName, $Bin, "migrate", "up")
    & (Get-Python) tools/seed.py
    Write-Host ""
    Write-Host "Dev is up at $AppUrl  (demo login: demo@gmail.com / Passw0rd!)"
  }
}

# ---- task dispatch ---------------------------------------------------------

function Get-HelpList {
  Write-Host ""
  Write-Host "Setup" -ForegroundColor White
  Write-Host "  bootstrap      Turnkey DEV: fresh stack -> S3 bucket+keys -> app -> migrate -> seed"
  Write-Host "  prod-bootstrap Turnkey PROD: fresh stack -> S3 bucket+keys -> app -> seed"
  Write-Host "  setup          One-time: create .env + garage config"
  Write-Host ""
  Write-Host "Dev stack" -ForegroundColor White
  Write-Host "  dev-up / dev-down / dev-restart / dev-attach"
  Write-Host "  dev-build / dev-build-clean / dev-logs [-S app] / dev-ps / dev-shell"
  Write-Host ""
  Write-Host "Prod stack" -ForegroundColor White
  Write-Host "  prod-up / prod-down / prod-build / prod-build-clean / prod-attach"
  Write-Host "  prod-logs [-S app] / prod-ps / prod-shell"
  Write-Host ""
  Write-Host "Database" -ForegroundColor White
  Write-Host "  migrate        Run DB migrations (inside the app container)"
  Write-Host "  seed           Seed rich demo data (RESETS app data)"
  Write-Host "  db-shell       Interactive psql shell in the postgres container"
  Write-Host ""
  Write-Host "Quality" -ForegroundColor White
  Write-Host "  fmt / tidy     go fmt / go mod tidy (inside the app container)"
  Write-Host ""
  Write-Host "Release" -ForegroundColor White
  Write-Host "  image-info / image-login / image-build / image-push / release [-Version x.y.z] [-AllowDirty]"
  Write-Host ""
  Write-Host "Cleanup" -ForegroundColor White
  Write-Host "  clean          Stop dev+prod stacks, drop named volumes (keeps data dirs)"
  Write-Host "  nuke           Remove containers, volumes, networks, built images + local data"
  Write-Host ""
}

switch ($Task) {

  # @ Setup
  "help" { Get-HelpList }
  "setup"         { New-EnvFile; New-GarageConfig; Write-Host "setup done - next: .\tasks.ps1 bootstrap" }
  "bootstrap"     { Invoke-Bootstrap -Prod $false }
  "dev-bootstrap" { Invoke-Bootstrap -Prod $false }
  "prod-bootstrap"{ Invoke-Bootstrap -Prod $true }

  # @ Dev stack
  "dev-up"          { Invoke-DC -Prod $false @("up", "-d", "--build") }
  "dev-attach"      { Invoke-DC -Prod $false @("up") }
  "dev-down"        { Invoke-DC -Prod $false @("down") }
  "dev-restart"     { Invoke-DC -Prod $false @("restart", $AppName) }
  "dev-build"       { Invoke-DC -Prod $false @("build", $AppName) }
  "dev-build-clean" { Invoke-DC -Prod $false @("build", "--no-cache", $AppName) }
  "dev-logs"        { if ($S) { Invoke-DC -Prod $false @("logs", "-f", $S) }
                      else    { Invoke-DC -Prod $false @("logs", "-f") } }
  "dev-shell"       { Invoke-DC -Prod $false @("exec", $AppName, "bash") }
  "dev-ps"          { Invoke-DC -Prod $false @("ps") }

  # @ Prod stack
  "prod-up"          { Invoke-DC -Prod $true @("up", "-d", "--build") }
  "prod-attach"      { Invoke-DC -Prod $true @("up") }
  "prod-down"        { Invoke-DC -Prod $true @("down") }
  "prod-build"       { Invoke-DC -Prod $true @("build") }
  "prod-build-clean" { Invoke-DC -Prod $true @("build", "--no-cache") }
  "prod-logs"        { if ($S) { Invoke-DC -Prod $true @("logs", "-f", $S) }
                       else    { Invoke-DC -Prod $true @("logs", "-f") } }
  "prod-shell"       { Invoke-DC -Prod $true @("exec", $AppName, "bash") }
  "prod-ps"          { Invoke-DC -Prod $true @("ps") }

  # @ Database
  "migrate" { Invoke-DC -Prod $false @("exec", "-T", $AppName, $Bin, "migrate", "up") }
  "seed"    { & (Get-Python) tools/seed.py }
  "db-shell"{ Invoke-DC -Prod $false @("exec", $PgService, "psql", "-U", "bureaucat", "-d", "bureaucat") }

  # @ Quality
  "fmt"  { Invoke-DC -Prod $false @("exec", "-T", $AppName, "go", "fmt", "./...") }
  "tidy" { Invoke-DC -Prod $false @("exec", "-T", $AppName, "go", "mod", "tidy") }

  # @ Release
  "image-info" {
    $sha = Invoke-NativeQuiet git @("rev-parse", "--short", "HEAD"); if (-not $sha) { $sha = "unknown" }
    Write-Host "image    : $Image"; Write-Host "tags     : $Version | sha-$sha | latest"
    Write-Host "platform : $Platform"; Write-Host "baked in : main.Version=$Version"
  }
  "image-login" { & docker login -u $DockerUser }
  "image-build" {
    $sha = Invoke-NativeQuiet git @("rev-parse", "--short", "HEAD"); if (-not $sha) { $sha = "unknown" }
    & docker build --platform $Platform --build-arg "VERSION=$Version" `
      -t "$($Image):$Version" -t "$($Image):sha-$sha" -t "$($Image):latest" .
  }
  "image-push" {
    $sha = Invoke-NativeQuiet git @("rev-parse", "--short", "HEAD"); if (-not $sha) { $sha = "unknown" }
    & docker push "$($Image):$Version"; & docker push "$($Image):sha-$sha"; & docker push "$($Image):latest"
  }
  "release" {
    $dirty = (& git status --porcelain) -join ""
    if ($dirty -and -not $AllowDirty) {
      $sha = Invoke-NativeQuiet git @("rev-parse", "--short", "HEAD")
      Write-Host "X working tree is dirty, so the sha-$sha tag would not match what you built."
      Write-Host "  commit first, or override with: .\tasks.ps1 release -AllowDirty"
      exit 1
    }
    $sha = Invoke-NativeQuiet git @("rev-parse", "--short", "HEAD"); if (-not $sha) { $sha = "unknown" }
    & docker build --platform $Platform --build-arg "VERSION=$Version" `
      -t "$($Image):$Version" -t "$($Image):sha-$sha" -t "$($Image):latest" .
    & docker push "$($Image):$Version"; & docker push "$($Image):sha-$sha"; & docker push "$($Image):latest"
    Write-Host "published $Image  [$Version, sha-$sha, latest]"
  }

  # @ Cleanup
  "clean" {
    Invoke-DC -Prod $false -Quiet @("down", "-v")
    Invoke-DC -Prod $true  -Quiet @("down", "-v")
    Write-Host "stopped + named volumes removed. Use 'nuke' to also remove built images + data."
  }
  "nuke" {
    Invoke-DC -Prod $false -Quiet @("down", "-v", "--remove-orphans", "--rmi", "local")
    Invoke-DC -Prod $true  -Quiet @("down", "-v", "--remove-orphans", "--rmi", "local")
    Remove-Item -Recurse -Force "garage", "postgres-data" -ErrorAction SilentlyContinue
    Write-Host "nuked: bureaucat dev+prod containers/volumes/networks/built-images + local data."
  }

  default { Write-Host "unknown task: '$Task'"; Get-HelpList; exit 1 }
}
