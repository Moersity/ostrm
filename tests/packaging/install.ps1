param([string]$Version,[string]$Arch)
$ErrorActionPreference='Stop'
$dest=Join-Path $env:RUNNER_TEMP 'ostrm-installed'
$env:OSTRM_SMOKE_DATA=Join-Path $env:RUNNER_TEMP 'ostrm-upgrade-data'
$setup="dist/ostrm_${Version}_windows_${Arch}_setup.exe"
for($i=0;$i -lt 2;$i++){
 $p=Start-Process $setup -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART',"/DIR=$dest") -Wait -PassThru
 if($p.ExitCode -ne 0){throw 'Install failed'}
 & node tests/packaging/smoke.mjs "$dest/ostrm.exe"
 if($LASTEXITCODE -ne 0){throw 'Installed binary failed'}
}
& "$dest/ostrm.exe" service install --listen 127.0.0.1:31117 --data-dir "$env:RUNNER_TEMP/ostrm-service-data"
if($LASTEXITCODE -ne 0){throw 'Service install failed'}
try {
 & "$dest/ostrm.exe" service start
 for($i=0;$i -lt 30;$i++){try {Invoke-RestMethod http://127.0.0.1:31117/health | Out-Null;break}catch{Start-Sleep 1}}
 Invoke-RestMethod http://127.0.0.1:31117/health | Out-Null
 & "$dest/ostrm.exe" service stop
} finally { & "$dest/ostrm.exe" service uninstall }
$p=Start-Process "$dest/unins000.exe" -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART') -Wait -PassThru
if($p.ExitCode -ne 0){throw 'Uninstall failed'}

if(!(Test-Path "$env:OSTRM_SMOKE_DATA/ostrm.db")){throw 'Uninstall removed user data'}
