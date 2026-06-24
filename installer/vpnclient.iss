; Inno Setup script for SAPN-VPN Windows client.
;
; Produces a single SAPN-VPN-Setup.exe bundling:
;   vpnclient.exe + xray.exe + sing-box.exe + wintun.dll + the built UI (ui\)
;   + geo databases (geo\).
;
; Build (from repo root, after `make release-staging`):
;   iscc installer\vpnclient.iss /DMyAppVersion=0.2.0
;
; The payload is taken from the staging dir produced by the Makefile
; (dist\stage by default; override with /DStageDir=...).

#ifndef MyAppVersion
  #define MyAppVersion "0.0.0-dev"
#endif

#ifndef StageDir
  #define StageDir "..\dist\stage"
#endif

#define MyAppName "SAPN VPN"
#define MyAppExeName "vpnclient.exe"
#define MyAppPublisher "SAPN"

[Setup]
AppId={{8C1F4D2E-7B3A-4E9C-9A21-5F0D6B8E1A77}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\SAPN VPN
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=..\dist
OutputBaseFilename=SAPN-VPN-Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
; Install per-machine into Program Files (TUN needs admin to run anyway, and a
; shared install matches the "run as administrator" flow).
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
ArchitecturesAllowed=x64compatible
UninstallDisplayIcon={app}\{#MyAppExeName}

[Languages]
Name: "en"; MessagesFile: "compiler:Default.isl"
Name: "ru"; MessagesFile: "compiler:Languages\Russian.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
; Optional: always launch elevated so the full-tunnel (TUN) mode works without
; the user having to right-click "Run as administrator".
Name: "runasadmin"; Description: "Always run as administrator (required for full-tunnel / TUN mode)"; GroupDescription: "Full tunnel:"; Flags: unchecked

[Files]
Source: "{#StageDir}\vpnclient.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\xray.exe";      DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\sing-box.exe";  DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\wintun.dll";    DestDir: "{app}"; Flags: ignoreversion
; Built React UI (served by the embedded control server).
Source: "{#StageDir}\ui\*";  DestDir: "{app}\ui";  Flags: ignoreversion recursesubdirs createallsubdirs
; Geo databases for xray (geoip.dat/geosite.dat) and sing-box rule-sets (*.srs).
Source: "{#StageDir}\geo\*"; DestDir: "{app}\geo"; Flags: ignoreversion recursesubdirs createallsubdirs skipifsourcedoesntexist

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
; shellexec (ShellExecuteEx) so the post-install launch honours the optional
; RUNASADMIN compatibility flag — CreateProcess can't elevate and fails with
; error 740 ("operation requires elevation") when run-as-admin was selected.
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: postinstall skipifsilent shellexec

[Code]
// If the user opted for "always run as administrator", set the per-exe
// compatibility flag so Windows elevates vpnclient.exe on every launch.
procedure CurStepChanged(CurStep: TSetupStep);
var
  ExePath: string;
begin
  if CurStep = ssPostInstall then
  begin
    if WizardIsTaskSelected('runasadmin') then
    begin
      ExePath := ExpandConstant('{app}\{#MyAppExeName}');
      RegWriteStringValue(HKLM,
        'SOFTWARE\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers',
        ExePath, 'RUNASADMIN');
    end;
  end;
end;

// Clean up the compatibility flag on uninstall.
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  ExePath: string;
begin
  if CurUninstallStep = usUninstall then
  begin
    ExePath := ExpandConstant('{app}\{#MyAppExeName}');
    RegDeleteValue(HKLM,
      'SOFTWARE\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers',
      ExePath);
  end;
end;
