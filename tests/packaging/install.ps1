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
$p=Start-Process "$dest/unins000.exe" -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART') -Wait -PassThru
if($p.ExitCode -ne 0){throw 'Uninstall failed'}

if(!(Test-Path "$env:OSTRM_SMOKE_DATA/ostrm.db")){throw 'Uninstall removed user data'}
