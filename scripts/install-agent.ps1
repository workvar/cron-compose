# CronCompose agent installer for Windows (Windows 10/11, Windows Server 2019+).
#
# Downloads the prebuilt agent .exe from the GitHub release, verifies its sha256
# against the release's checksums.json, enrolls it with the control plane, and
# registers it as the croncompose-agent Windows service (LocalSystem, automatic
# start, restart on failure). The agent then keeps itself up to date.
#
# Run from an Administrator PowerShell. The control plane UI prints this line with
# your values filled in:
#
#   $env:TOKEN='<token>'; $env:AGENT_ENROLL_HTTP='https://cc.example.com/api'; `
#   $env:AGENT_GRPC_ADDR='cc.example.com:9090'; `
#   irm https://github.com/workvar/cron-compose/releases/latest/download/install-agent.ps1 | iex
#
# Settings (environment variables):
#   TOKEN                one-time enrollment token from the UI (required)
#   AGENT_ENROLL_HTTP    public REST base for enroll, e.g. https://cc.example.com/api
#   AGENT_GRPC_ADDR      mTLS gRPC endpoint host:port (or a bare host for port 443)
#   AGENT_GRPC_SNI       server name to verify (defaults to the host part of ADDR)
#   AGENT_VERSION        release tag to install; defaults to this script's baked tag
#   GITHUB_REPO          owner/repo releases come from; default workvar/cron-compose
#   INSTALL_DIR          default "$env:ProgramFiles\CronCompose"
#   DATA_DIR             default "$env:ProgramData\CronCompose"
#   AGENT_AUTO_UPDATE    1 (default) lets the agent install new releases by itself
#   UNINSTALL            1 removes the service and binary (add PURGE=1 to also
#                        delete the data folder, including the agent's identity)
#
# Baked release: __VERSION__

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor 3072

$ServiceName = 'croncompose-agent'

function Say($msg) { Write-Host "==> $msg" }
function Warn($msg) { Write-Warning $msg }
# Fail throws instead of calling exit: under `irm | iex` an exit would close the
# operator's PowerShell window along with the script.
function Fail($msg) { throw "CronCompose install: $msg" }
function EnvOr($name, $default) {
  $v = [Environment]::GetEnvironmentVariable($name, 'Process')
  if ([string]::IsNullOrWhiteSpace($v)) { return $default }
  return $v.Trim()
}

function Install-CronComposeAgent {
  # --- preflight ---------------------------------------------------------------

  $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
  if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Fail 'run this from an Administrator PowerShell (right-click PowerShell > Run as administrator)'
  }

  $InstallDir = EnvOr 'INSTALL_DIR' (Join-Path $env:ProgramFiles 'CronCompose')
  $DataDir    = EnvOr 'DATA_DIR' (Join-Path $env:ProgramData 'CronCompose')
  $EnvFile    = Join-Path (Join-Path $env:ProgramData 'CronCompose') 'agent.env'
  $Exe        = Join-Path $InstallDir 'croncompose-agent.exe'

  function Stop-Agent {
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($svc -and $svc.Status -ne 'Stopped') {
      Say "stopping the running $ServiceName service"
      Stop-Service -Name $ServiceName -Force
      (Get-Service -Name $ServiceName).WaitForStatus('Stopped', [TimeSpan]::FromSeconds(30))
    }
  }

  if ((EnvOr 'UNINSTALL' '0') -eq '1') {
    Stop-Agent
    if (Test-Path $Exe) { & $Exe service uninstall } else { sc.exe delete $ServiceName | Out-Null }
    Remove-Item -Recurse -Force $InstallDir -ErrorAction SilentlyContinue
    if ((EnvOr 'PURGE' '0') -eq '1') {
      Remove-Item -Recurse -Force $DataDir -ErrorAction SilentlyContinue
      Remove-Item -Force $EnvFile -ErrorAction SilentlyContinue
    }
    Say 'CronCompose agent removed'
    return
  }

  $Token    = EnvOr 'TOKEN' ''
  $EnrollUrl = EnvOr 'AGENT_ENROLL_HTTP' (EnvOr 'CONTROL_PLANE_HTTP' '')
  $GrpcAddr = EnvOr 'AGENT_GRPC_ADDR' (EnvOr 'CONTROL_PLANE_ADDR' '')
  if (-not $Token)     { Fail 'TOKEN is required (copy the install line from the CronCompose UI)' }
  if (-not $EnrollUrl) { Fail 'AGENT_ENROLL_HTTP is required' }
  if (-not $GrpcAddr)  { Fail 'AGENT_GRPC_ADDR is required' }
  $Sni = EnvOr 'AGENT_GRPC_SNI' (EnvOr 'CONTROL_PLANE_SNI' ($GrpcAddr -replace ':\d+$', ''))

  # CI replaces __VERSION__ / __REPO__ when attaching this file to a GitHub release.
  $Repo = EnvOr 'GITHUB_REPO' '__REPO__'
  if ($Repo -eq ('__' + 'REPO__')) { $Repo = 'workvar/cron-compose' }
  $Version = EnvOr 'AGENT_VERSION' '__VERSION__'
  if ($Version -eq ('__' + 'VERSION__') -or $Version -eq 'latest') {
    Say "resolving latest release of $Repo"
    $Version = (Invoke-RestMethod -Headers @{ 'User-Agent' = 'croncompose-installer' } `
      -Uri "https://api.github.com/repos/$Repo/releases/latest").tag_name
    if (-not $Version) { Fail "could not read the latest release tag of $Repo" }
  }

  $arch = $env:PROCESSOR_ARCHITEW6432
  if (-not $arch) { $arch = $env:PROCESSOR_ARCHITECTURE }
  switch ($arch) {
    'AMD64' { $GoArch = 'amd64' }
    'ARM64' { $GoArch = 'arm64' }
    default { Fail "unsupported CPU architecture: $arch (the agent ships for x64 and ARM64)" }
  }

  $build = [Environment]::OSVersion.Version.Build
  if ($build -lt 17763) {
    Warn "Windows build $build predates 1809 / Server 2019: jobs and deploys work, but the web terminal needs ConPTY (build 17763+)."
  }

  # --- download and verify -----------------------------------------------------

  $Asset = "croncompose-agent-windows-$GoArch.exe"
  $Base  = "https://github.com/$Repo/releases/download/$Version"
  $Tmp   = Join-Path ([IO.Path]::GetTempPath()) ("croncompose-" + [Guid]::NewGuid().ToString('N'))
  New-Item -ItemType Directory -Path $Tmp | Out-Null
  try {
    Say "downloading $Asset ($Version)"
    Invoke-WebRequest -UseBasicParsing -Uri "$Base/$Asset" -OutFile (Join-Path $Tmp $Asset)
    $raw = (Invoke-WebRequest -UseBasicParsing -Uri "$Base/checksums.json").Content
    if ($raw -is [byte[]]) { $raw = [Text.Encoding]::UTF8.GetString($raw) }
    $sums = $raw | ConvertFrom-Json
    $want = $sums."windows/$GoArch"
    if (-not $want) { Fail "release $Version has no checksum for windows/$GoArch" }
    $got = (Get-FileHash -Algorithm SHA256 (Join-Path $Tmp $Asset)).Hash
    if ($got -ne $want.ToUpper()) { Fail "checksum mismatch for ${Asset}: got $got, expected $want" }
    Say 'sha256 verified'

    Stop-Agent
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -Force (Join-Path $Tmp $Asset) $Exe
  } finally {
    Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
  }

  # --- data folder and settings -------------------------------------------------

  New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
  New-Item -ItemType Directory -Force -Path (Split-Path $EnvFile) | Out-Null
  # The data folder holds the agent's private key: SYSTEM and Administrators only.
  # SIDs keep this working on non-English Windows.
  icacls $DataDir /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' | Out-Null

  $autoUpdate = EnvOr 'AGENT_AUTO_UPDATE' '1'
  $lines = @(
    '# CronCompose agent settings, read by the croncompose-agent service at start.',
    "AGENT_ENROLL_HTTP=$EnrollUrl",
    "AGENT_GRPC_ADDR=$GrpcAddr",
    "AGENT_GRPC_SNI=$Sni",
    "DATA_DIR=$DataDir",
    "AGENT_UPDATE_REPO=$Repo",
    "AGENT_AUTO_UPDATE=$autoUpdate",
    "PM2_HOME=$(Join-Path $DataDir 'pm2')"
  )
  $mode = EnvOr 'AGENT_GRPC_MODE' ''
  if ($mode) { $lines += "AGENT_GRPC_MODE=$mode" }
  # UTF-8 without BOM so the file reads the same everywhere.
  [IO.File]::WriteAllLines($EnvFile, $lines, (New-Object Text.UTF8Encoding($false)))
  icacls $EnvFile /inheritance:r /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' | Out-Null

  # --- enroll and start -------------------------------------------------------

  Say 'enrolling with the control plane'
  $env:AGENT_ENROLL_HTTP = $EnrollUrl
  $env:AGENT_GRPC_ADDR = $GrpcAddr
  $env:AGENT_GRPC_SNI = $Sni
  $env:DATA_DIR = $DataDir
  & $Exe enroll "--token=$Token"
  if ($LASTEXITCODE -ne 0) { Fail 'enrollment failed (is the token still valid and AGENT_ENROLL_HTTP reachable?)' }

  Say "registering the $ServiceName service"
  & $Exe service install
  if ($LASTEXITCODE -ne 0) { Fail 'could not register the Windows service' }
  & $Exe service start
  if ($LASTEXITCODE -ne 0) { Fail "the service did not start; see $DataDir\logs\agent.log" }

  # --- soft checks -------------------------------------------------------------

  $hasGit = [bool](Get-Command git -ErrorAction SilentlyContinue)
  if (-not $hasGit) {
    Warn 'Git for Windows is not installed. Deploys and bash jobs need it (it provides git and bash):'
    Warn '  winget install --id Git.Git -e --scope machine   (then: Restart-Service croncompose-agent)'
  }
  try {
    $hostPart = $GrpcAddr -replace ':\d+$', ''
    $port = 443
    if ($GrpcAddr -match ':(\d+)$') { $port = [int]$Matches[1] }
    $ok = Test-NetConnection -ComputerName $hostPart -Port $port -InformationLevel Quiet -WarningAction SilentlyContinue
    if (-not $ok) { Warn "cannot open a TCP connection to ${hostPart}:$port; the agent will keep retrying until it can." }
  } catch { }

  Say "CronCompose agent $Version installed"
  Write-Host "    binary:   $Exe"
  Write-Host "    data:     $DataDir"
  Write-Host "    settings: $EnvFile"
  Write-Host "    logs:     $DataDir\logs\agent.log"
  Write-Host "    service:  Get-Service $ServiceName"
}

Install-CronComposeAgent
