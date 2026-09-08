param([string]$Version,[string]$Arch)
$ErrorActionPreference='Stop'
$installer=Join-Path $env:RUNNER_TEMP 'innosetup.exe'
Invoke-WebRequest 'https://github.com/jrsoftware/issrc/releases/download/is-7_1_0/innosetup-7.1.0-x64.exe' -OutFile $installer
if ((Get-FileHash $installer -Algorithm SHA256).Hash.ToLower() -ne '0362a383ed217d4c4239b5933866dd96d3eb2102737da92f80f6057a4b40df2f') { throw 'Inno checksum mismatch' }
$tool=Join-Path $env:RUNNER_TEMP 'inno'
$p=Start-Process $installer -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART',"/DIR=$tool") -Wait -PassThru
if($p.ExitCode -ne 0){throw 'Inno install failed'}
& "$tool\ISCC.exe" "/DAppVersion=$Version" "/DArch=$Arch" packaging/windows/installer.iss
if($LASTEXITCODE -ne 0){throw 'ISCC failed'}
if($env:WINDOWS_SIGN_CERT){
  $cert=Get-PfxCertificate $env:WINDOWS_SIGN_CERT
  $signature=Set-AuthenticodeSignature -FilePath "dist/ostrm_${Version}_windows_${Arch}_setup.exe" -Certificate $cert -TimestampServer 'http://timestamp.digicert.com'
  if($signature.Status -ne 'Valid'){throw 'Signature invalid'}
}
