<#
.SYNOPSIS
  CronCompose interactive installer for Windows.

.DESCRIPTION
  Builds the control plane (Postgres-backed Go API + Next.js web UI) from source,
  applies database migrations, and starts everything. Run from a checkout of the repo:

      ./install/install.ps1

  The per-server agent is NOT installed on Windows: it relies on a Unix shell and
  Unix process APIs. Run the agent on a Linux/macOS host (see scripts/install-agent.sh)
  and point it at this control plane.

.PARAMETER NonInteractive
  Accept defaults / CC_* environment variables without prompting.

.PARAMETER NoWeb
  Do not build or run the web UI (API-only install).

.PARAMETER RuntimeDir
  Where to keep logs, pids, and TLS material.
#>
[CmdletBinding()]
param(
  [switch]$NonInteractive,
  [switch]$NoWeb,
  [string]$RuntimeDir
)

$ErrorActionPreference = 'Stop'
$script:NonInteractive = [bool]$NonInteractive
$script:EnableWeb = -not $NoWeb

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot  = Split-Path -Parent $ScriptDir
. "$ScriptDir/lib/Common.ps1"

# All resolved configuration lives here.
$Cfg = @{}

function Get-CcEnv($name, $default) {
  $v = [Environment]::GetEnvironmentVariable($name)
  if ([string]::IsNullOrEmpty($v)) { return $default } else { return $v }
}

# --- preflight --------------------------------------------------------------

function Test-VersionGe($have, $want) {
  $h = $have.Split('.'); $w = $want.Split('.')
  $h0 = [int]$h[0]; $w0 = [int]$w[0]
  if ($h0 -ne $w0) { return $h0 -gt $w0 }
  $h1 = if ($h.Length -gt 1) { [int]$h[1] } else { 0 }
  $w1 = if ($w.Length -gt 1) { [int]$w[1] } else { 0 }
  return $h1 -ge $w1
}

function Invoke-Preflight {
  Write-Step "Checking prerequisites"
  Write-Ok "platform: windows/$($env:PROCESSOR_ARCHITECTURE)"

  if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Die "Go is required (need 1.25+). Install from https://go.dev/dl/ and re-run."
  }
  $gov = ((go version) -split '\s+')[2] -replace '^go', ''
  if (Test-VersionGe $gov '1.25') { Write-Ok "go $gov" } else { Die "Go $gov found but 1.25+ is required." }

  if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
    Die "Node.js is required (need 20+). Install from https://nodejs.org/ and re-run."
  }
  $nodev = (node --version) -replace '^v', ''
  if (Test-VersionGe $nodev '20') { Write-Ok "node $nodev" } else { Die "Node $nodev found but 20+ is required." }
  if (-not (Get-Command npm -ErrorAction SilentlyContinue)) { Die "npm not found alongside Node." }

  $Cfg.HavePsql   = [bool](Get-Command psql -ErrorAction SilentlyContinue)
  $Cfg.HaveDocker = [bool](Get-Command docker -ErrorAction SilentlyContinue)
  if ($Cfg.HavePsql)   { Write-Ok "psql detected (enables auto-creating a local database)" }
  if ($Cfg.HaveDocker) { Write-Ok "docker detected (enables a containerized Postgres)" }
}

# --- configure --------------------------------------------------------------

function Invoke-Configure {
  Write-Step "Where to keep runtime state"
  if (-not $RuntimeDir) { $RuntimeDir = Get-CcEnv 'CC_RUNTIME_DIR' "$RepoRoot\.run" }
  $Cfg.RuntimeDir = Read-Default "Runtime directory (logs, pids, TLS)" $RuntimeDir
  # Accepts a bare host, host:port, or a full URL; Split-AdvertiseHost keeps the raw
  # value from being pasted into "http://$Advertise:$Port".
  $advRaw = Read-Default "Advertise host/IP or URL (what browsers use to reach this box)" (Get-CcEnv 'CC_ADVERTISE_HOST' 'localhost')
  $Cfg.Adv       = Split-AdvertiseHost $advRaw
  $Cfg.Advertise = $Cfg.Adv.Host

  Write-Step "Choose ports (a free one is suggested for each)"
  $Cfg.WebPort  = Read-Port "Web UI port"     ([int](Get-CcEnv 'CC_WEB_PORT' '3000'))
  $Cfg.ApiPort  = Read-Port "REST API port"   ([int](Get-CcEnv 'CC_API_PORT' '8080'))
  $Cfg.GrpcPort = Read-Port "Agent gRPC port" ([int](Get-CcEnv 'CC_GRPC_PORT' '9090'))
  Write-Ok "web=$($Cfg.WebPort)  api=$($Cfg.ApiPort)  grpc=$($Cfg.GrpcPort)"

  Write-Step "Seed administrator account"
  $Cfg.AdminEmail = Read-Default "Admin email" (Get-CcEnv 'CC_ADMIN_EMAIL' 'admin@example.com')
  $Cfg.AdminPass  = Read-Secret "Admin password (blank = generate one)" (Get-CcEnv 'CC_ADMIN_PASSWORD' '')
  if ([string]::IsNullOrWhiteSpace($Cfg.AdminPass)) {
    $Cfg.AdminPass = New-HexSecret 12; $Cfg.AdminPassGen = $true
    Write-Ok "generated admin password: $($Cfg.AdminPass)"
  }

  Write-Step "Generating secrets"
  $Cfg.SessionSecret = New-HexSecret 32
  $Cfg.SecretsKey    = New-HexSecret 32
  Write-Ok "SESSION_SECRET and SECRETS_MASTER_KEY generated"
  $Cfg.TlsHosts = Get-CcEnv 'CC_TLS_HOSTS' "localhost,127.0.0.1,$($Cfg.Advertise)"
  $Cfg.LogLevel = Read-Default "Log level (debug|info|warn|error)" (Get-CcEnv 'CC_LOG_LEVEL' 'info')

  Configure-Database
  Configure-Oidc

  Write-Step "Writing configuration"
  $Cfg.ApiBase  = "http://127.0.0.1:$($Cfg.ApiPort)/api/v1"
  Write-EnvFile
}

function Configure-Database {
  Write-Step "Database (PostgreSQL)"
  $method = Get-CcEnv 'CC_DB_METHOD' 'existing'
  if (-not $script:NonInteractive) {
    Write-Info "1) Use an existing Postgres (enter a connection string)"
    if ($Cfg.HavePsql)   { Write-Info "2) Auto-create a local database with psql" }
    if ($Cfg.HaveDocker) { Write-Info "3) Run Postgres in Docker (docker-compose.yml)" }
    switch (Read-Default "Select" "1") {
      "2" { if ($Cfg.HavePsql)   { $method = 'psql' }   else { $method = 'existing' } }
      "3" { if ($Cfg.HaveDocker) { $method = 'docker' } else { $method = 'existing' } }
      default { $method = 'existing' }
    }
  }
  $Cfg.DbMethod = $method
  switch ($method) {
    'docker' {
      $Cfg.DbPort = Read-Port "Host port to expose Postgres on" ([int](Get-CcEnv 'CC_DB_PORT' '5432'))
      $Cfg.DbUser = 'croncompose'
      $Cfg.DatabaseUrl = "postgres://croncompose:croncompose@127.0.0.1:$($Cfg.DbPort)/croncompose?sslmode=disable"
      Write-Ok "will start Postgres in Docker on 127.0.0.1:$($Cfg.DbPort)"
    }
    'psql' {
      $Cfg.DbHost = Read-Default "Postgres host" (Get-CcEnv 'CC_DB_HOST' 'localhost')
      $Cfg.DbPort = Read-Default "Postgres port" (Get-CcEnv 'CC_DB_PORT' '5432')
      $Cfg.DbSuper = Read-Default "Superuser to create role/db with" (Get-CcEnv 'CC_DB_SUPERUSER' 'postgres')
      $Cfg.DbSuperPass = Read-Secret "Superuser password (blank if trust auth)" (Get-CcEnv 'CC_DB_SUPER_PASS' '')
      $Cfg.DbName = Read-Default "New database name" (Get-CcEnv 'CC_DB_NAME' 'croncompose')
      $Cfg.DbUser = Read-Default "New database role" (Get-CcEnv 'CC_DB_USER' 'croncompose')
      $Cfg.DbPass = Read-Secret "Password for new role (blank = generate)" (Get-CcEnv 'CC_DB_PASS' '')
      if ([string]::IsNullOrWhiteSpace($Cfg.DbPass)) { $Cfg.DbPass = New-HexSecret 12; Write-Ok "generated db password: $($Cfg.DbPass)" }
      $Cfg.DatabaseUrl = "postgres://$($Cfg.DbUser):$($Cfg.DbPass)@$($Cfg.DbHost):$($Cfg.DbPort)/$($Cfg.DbName)?sslmode=disable"
    }
    default {
      $Cfg.DatabaseUrl = Read-Default "DATABASE_URL" (Get-CcEnv 'CC_DATABASE_URL' 'postgres://croncompose:croncompose@localhost:5432/croncompose?sslmode=disable')
    }
  }
}

function Configure-Oidc {
  if (Confirm-Yes "Configure OIDC single sign-on now?" 'n') {
    Write-Step "OIDC SSO"
    $Cfg.OidcIssuer   = Read-Default "OIDC issuer URL" ''
    $Cfg.OidcClientId = Read-Default "OIDC client id" ''
    $Cfg.OidcSecret   = Read-Secret  "OIDC client secret (blank for public client)" ''
    $Cfg.OidcRedirect = Read-Default "OIDC redirect URL" "$(Get-PublicBaseUrl $Cfg.Adv $Cfg.ApiPort)/api/v1/auth/oidc/callback"
    $Cfg.OidcRole     = Read-Default "Default role for new SSO users" 'viewer'
  }
}

function Write-EnvFile {
  $Cfg.EnvFile = "$RepoRoot\.env"
  $lines = @(
    "# Generated by install.ps1 on $(Get-Date). Contains secrets - keep private."
    "APP_ENV=prod"
    "LOG_LEVEL=$($Cfg.LogLevel)"
    "DATABASE_URL=$($Cfg.DatabaseUrl)"
    "HTTP_ADDR=:$($Cfg.ApiPort)"
    "GRPC_ADDR=:$($Cfg.GrpcPort)"
    "TLS_DIR=$($Cfg.RuntimeDir)\tls"
    "TLS_HOSTS=$($Cfg.TlsHosts)"
    "SESSION_SECRET=$($Cfg.SessionSecret)"
    "SECRETS_MASTER_KEY=$($Cfg.SecretsKey)"
    "SEED_ADMIN_EMAIL=$($Cfg.AdminEmail)"
    "SEED_ADMIN_PASSWORD=$($Cfg.AdminPass)"
    "PUBLIC_HTTP_URL=$(Get-PublicBaseUrl $Cfg.Adv $Cfg.ApiPort)/api/v1"
    "PUBLIC_GRPC_ADDR=$($Cfg.Advertise):$($Cfg.GrpcPort)"
    "PORT=$($Cfg.WebPort)"
    "API_BASE=$($Cfg.ApiBase)"
  )
  if ($Cfg.OidcIssuer) {
    $lines += @(
      "OIDC_ISSUER_URL=$($Cfg.OidcIssuer)"
      "OIDC_CLIENT_ID=$($Cfg.OidcClientId)"
      "OIDC_CLIENT_SECRET=$($Cfg.OidcSecret)"
      "OIDC_REDIRECT_URL=$($Cfg.OidcRedirect)"
      "OIDC_DEFAULT_ROLE=$($Cfg.OidcRole)"
    )
  }
  $lines += @(
    "CC_WEB_PORT=$($Cfg.WebPort)"
    "CC_API_PORT=$($Cfg.ApiPort)"
    "CC_GRPC_PORT=$($Cfg.GrpcPort)"
    "CC_RUNTIME_DIR=$($Cfg.RuntimeDir)"
    "CC_ADVERTISE_HOST=$($Cfg.Advertise)"
    "CC_ENABLE_WEB=$([int][bool]$script:EnableWeb)"
  )
  Set-Content -Path $Cfg.EnvFile -Value $lines -Encoding ascii
  Write-Ok "wrote $($Cfg.EnvFile)"
}

# --- database ---------------------------------------------------------------

function Invoke-Database {
  switch ($Cfg.DbMethod) {
    'docker' {
      Write-Step "Starting Postgres in Docker"
      Push-Location $RepoRoot
      try {
        docker compose -f docker-compose.yml up -d postgres
        if ($LASTEXITCODE -ne 0) { Die "failed to start Postgres container" }
        Write-Info "waiting for Postgres to accept connections..."
        for ($i = 0; $i -lt 60; $i++) {
          docker compose -f docker-compose.yml exec -T postgres pg_isready -U $Cfg.DbUser *> $null
          if ($LASTEXITCODE -eq 0) { Write-Ok "Postgres is ready"; break }
          Start-Sleep -Seconds 1
        }
      } finally { Pop-Location }
    }
    'psql' {
      Write-Step "Creating database role and owner via psql"
      $superUrl = "postgres://$($Cfg.DbSuper)@$($Cfg.DbHost):$($Cfg.DbPort)/postgres?sslmode=disable"
      if ($Cfg.DbSuperPass) { $env:PGPASSWORD = $Cfg.DbSuperPass }
      $roleExists = (psql $superUrl -tAc "select 1 from pg_roles where rolname='$($Cfg.DbUser)'") 2>$null
      if ($roleExists -ne '1') {
        psql $superUrl -v ON_ERROR_STOP=1 -c "create role ""$($Cfg.DbUser)"" login password '$($Cfg.DbPass)'" | Out-Null
        Write-Ok "created role $($Cfg.DbUser)"
      } else {
        psql $superUrl -c "alter role ""$($Cfg.DbUser)"" login password '$($Cfg.DbPass)'" | Out-Null
        Write-Ok "role $($Cfg.DbUser) already existed (password updated)"
      }
      $dbExists = (psql $superUrl -tAc "select 1 from pg_database where datname='$($Cfg.DbName)'") 2>$null
      if ($dbExists -ne '1') {
        psql $superUrl -v ON_ERROR_STOP=1 -c "create database ""$($Cfg.DbName)"" owner ""$($Cfg.DbUser)""" | Out-Null
        Write-Ok "created database $($Cfg.DbName)"
      } else { Write-Ok "database $($Cfg.DbName) already existed" }
      Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue
    }
    default { Write-Step "Database"; Write-Ok "using existing Postgres at the supplied DATABASE_URL" }
  }
}

function Invoke-Migrate {
  Write-Step "Applying database migrations"
  $migrate = "$RepoRoot\control-plane\bin\migrate.exe"
  if (-not (Test-Path $migrate)) { Die "migrate tool not built ($migrate)" }
  $env:DATABASE_URL = $Cfg.DatabaseUrl
  & $migrate -dir "$RepoRoot\migrations"
  if ($LASTEXITCODE -ne 0) { Die "migrations failed" }
  Write-Ok "schema is up to date"
}

# --- build ------------------------------------------------------------------

function Invoke-Build {
  Write-Step "Building Go binaries"
  # Use only the locally installed Go; never download a different Go toolchain.
  $env:GOTOOLCHAIN = 'local'
  Push-Location "$RepoRoot\control-plane"
  try {
    go build -o bin/control-plane.exe ./cmd/server; if ($LASTEXITCODE) { Die "control-plane build failed" }; Write-Ok "control-plane"
    go build -o bin/migrate.exe ./cmd/migrate;      if ($LASTEXITCODE) { Die "migrate build failed" };       Write-Ok "migrate"
  } finally { Pop-Location }
  Push-Location "$RepoRoot\cli"
  try { go build -o bin/cc.exe ./cmd/cc; if ($LASTEXITCODE) { Die "cli build failed" }; Write-Ok "cc CLI" }
  finally { Pop-Location }

  if (-not $script:EnableWeb) { Write-Dim "skipping web build (-NoWeb)"; return }
  Write-Step "Building the web UI (npm install + next build)"
  Push-Location "$RepoRoot\web"
  try {
    npm install --no-audit --no-fund; if ($LASTEXITCODE) { Die "npm install failed" }; Write-Ok "dependencies installed"
    $env:API_BASE = $Cfg.ApiBase; $env:NEXT_TELEMETRY_DISABLED = '1'
    npm run build; if ($LASTEXITCODE) { Die "next build failed" }
    # Stage the standalone runtime (mirrors web/Dockerfile).
    $std = "$RepoRoot\web\.next\standalone"
    Remove-Item -Recurse -Force "$std\.next\static" -ErrorAction SilentlyContinue
    Copy-Item -Recurse -Force "$RepoRoot\web\.next\static" "$std\.next\static"
    if (Test-Path "$RepoRoot\web\public") {
      Remove-Item -Recurse -Force "$std\public" -ErrorAction SilentlyContinue
      Copy-Item -Recurse -Force "$RepoRoot\web\public" "$std\public"
    }
    Write-Ok "web UI built (standalone runtime staged)"
  } finally { Pop-Location }
}

# --- services ---------------------------------------------------------------

function Write-CtlScript {
  Write-Step "Generating control script"
  $Cfg.Ctl = "$RepoRoot\croncompose-ctl.ps1"
  $body = @'
# CronCompose process manager (generated by install.ps1).
# Usage: ./croncompose-ctl.ps1 [start|stop|restart|status|logs] [control-plane|web]
param([string]$Action = 'status', [string]$Service = 'control-plane')
$ErrorActionPreference = 'SilentlyContinue'
$Here = $PSScriptRoot
Get-Content "$Here\.env" | ForEach-Object {
  if ($_ -match '^\s*#' -or $_ -notmatch '=') { return }
  $i = $_.IndexOf('='); $k = $_.Substring(0, $i); $v = $_.Substring($i + 1)
  [Environment]::SetEnvironmentVariable($k.Trim(), $v, 'Process')
}
$Runtime = if ($env:CC_RUNTIME_DIR) { $env:CC_RUNTIME_DIR } else { "$Here\.run" }
$Logs = "$Runtime\logs"; $Run = "$Runtime\run"
New-Item -ItemType Directory -Force -Path $Logs, $Run, "$Runtime\tls" | Out-Null
$ApiPort = if ($env:CC_API_PORT) { $env:CC_API_PORT } else { '8080' }
$WebPort = if ($env:CC_WEB_PORT) { $env:CC_WEB_PORT } else { '3000' }

function Alive($pf) { if (Test-Path $pf) { $null -ne (Get-Process -Id (Get-Content $pf) -ErrorAction SilentlyContinue) } else { $false } }
function StartProc($name, $exe, $argv, $workdir) {
  $pf = "$Run\$name.pid"
  if (Alive $pf) { Write-Host "  $name already running (pid $(Get-Content $pf))"; return }
  $opts = @{ FilePath = $exe; WorkingDirectory = $workdir; PassThru = $true; WindowStyle = 'Hidden';
             RedirectStandardOutput = "$Logs\$name.out.log"; RedirectStandardError = "$Logs\$name.err.log" }
  if ($argv -and $argv.Count -gt 0) { $opts.ArgumentList = $argv }
  $p = Start-Process @opts
  $p.Id | Set-Content $pf
  Write-Host "  started $name (pid $($p.Id))"
}
function DoStart {
  Write-Host "Starting CronCompose..."
  StartProc 'control-plane' "$Here\control-plane\bin\control-plane.exe" @() $Here
  for ($i=0; $i -lt 60; $i++) { try { Invoke-WebRequest -UseBasicParsing "http://127.0.0.1:$ApiPort/healthz" -TimeoutSec 3 | Out-Null; break } catch { if ($_.Exception.Response) { break } }; Start-Sleep 1 }
  Write-Host "  control-plane on :$ApiPort"
  if (($env:CC_ENABLE_WEB -ne '0') -and (Test-Path "$Here\web\.next\standalone\server.js")) {
    $env:PORT = $WebPort; $env:HOSTNAME = '0.0.0.0'
    StartProc 'web' 'node' @('server.js') "$Here\web\.next\standalone"
    Write-Host "  web on :$WebPort"
  }
}
function StopOne($name) { $pf = "$Run\$name.pid"; if (Alive $pf) { Stop-Process -Id (Get-Content $pf) -Force; Remove-Item $pf; Write-Host "  stopped $name" } else { Write-Host "  $name not running" } }
function DoStatus { foreach ($s in 'control-plane','web') { $pf = "$Run\$s.pid"; if (Alive $pf) { Write-Host "  ${s}: running (pid $(Get-Content $pf))" } else { Write-Host "  ${s}: stopped" } } }

switch ($Action) {
  'start'   { DoStart }
  'stop'    { Write-Host "Stopping CronCompose..."; StopOne 'web'; StopOne 'control-plane' }
  'restart' { StopOne 'web'; StopOne 'control-plane'; Start-Sleep 1; DoStart }
  'status'  { DoStatus }
  'logs'    { Get-Content "$Logs\$Service.err.log", "$Logs\$Service.out.log" -Wait -Tail 100 }
  default   { Write-Host "usage: ./croncompose-ctl.ps1 [start|stop|restart|status|logs] [control-plane|web]" }
}
'@
  Set-Content -Path $Cfg.Ctl -Value $body -Encoding ascii
  Write-Ok "wrote $($Cfg.Ctl)"
}

function Start-Stack {
  Write-Step "Starting services"
  New-Item -ItemType Directory -Force -Path "$($Cfg.RuntimeDir)\logs", "$($Cfg.RuntimeDir)\run", "$($Cfg.RuntimeDir)\tls" | Out-Null
  # Run the generated control script in the current PowerShell host (works under both
  # pwsh 7+ and Windows PowerShell 5.1). Start-Process detaches the daemons.
  & $Cfg.Ctl start
  if (-not (Wait-Http "http://127.0.0.1:$($Cfg.ApiPort)/healthz")) { Write-Warnn "control-plane health check timed out (see $($Cfg.RuntimeDir)\logs)" }
}

function Show-Summary {
  $web = Get-PublicBaseUrl $Cfg.Adv $Cfg.WebPort
  Write-Host "`n============ CronCompose is installed and running ============" -ForegroundColor Green
  Write-Info ""
  Write-Info "Web UI:     $web"
  Write-Info "REST API:   $(Get-PublicBaseUrl $Cfg.Adv $Cfg.ApiPort)/api/v1   (health: /healthz)"
  Write-Info "Agent gRPC: $($Cfg.Advertise):$($Cfg.GrpcPort)   (enroll agents from Linux/macOS hosts)"
  Write-Info ""
  Write-Info "Sign in with:  $($Cfg.AdminEmail)"
  if ($Cfg.AdminPassGen) { Write-Info "Password:      $($Cfg.AdminPass)   (generated - save it now)" }
  Write-Info ""
  Write-Info "Manage the stack:"
  Write-Info "  ./croncompose-ctl.ps1 status"
  Write-Info "  ./croncompose-ctl.ps1 logs web"
  Write-Info "  ./croncompose-ctl.ps1 restart"
  Write-Info "  ./croncompose-ctl.ps1 stop"
  Write-Info ""
  Write-Info "Config + secrets are in .env. Runtime state in: $($Cfg.RuntimeDir)"
  Write-Dim "The per-server agent is Linux/macOS only; run it on target hosts and dial this control plane."
}

# --- main -------------------------------------------------------------------

Write-Host "CronCompose installer (Windows)" -ForegroundColor Cyan
Write-Dim "Builds and runs the control plane from source. Agent is Linux/macOS only."
Invoke-Preflight
Invoke-Configure
Invoke-Database
Invoke-Build
Invoke-Migrate
Write-CtlScript
Start-Stack
Show-Summary
