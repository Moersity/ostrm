#ifndef AppVersion
#define AppVersion "3.0.0-dev"
#endif
#ifndef Arch
#define Arch "amd64"
#endif
[Setup]
AppId=OStrmGo
AppName=OStrm Go
AppVersion={#AppVersion}
DefaultDirName={localappdata}\Programs\OStrm
DefaultGroupName=OStrm
PrivilegesRequired=lowest
OutputDir=..\..\dist
OutputBaseFilename=ostrm_{#AppVersion}_windows_{#Arch}_setup
Compression=lzma2
SolidCompression=yes
CloseApplications=yes
#if Arch == "arm64"
ArchitecturesAllowed=arm64
ArchitecturesInstallIn64BitMode=arm64
#else
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
#endif
[Files]
Source: "..\..\dist\ostrm_{#AppVersion}_windows_{#Arch}\ostrm.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\LICENSE"; DestDir: "{app}"
[Icons]
Name: "{group}\OStrm"; Filename: "{app}\ostrm.exe"
Name: "{group}\卸载 OStrm"; Filename: "{uninstallexe}"
[Run]
Filename: "{app}\ostrm.exe"; Description: "启动 OStrm"; Flags: nowait postinstall skipifsilent
