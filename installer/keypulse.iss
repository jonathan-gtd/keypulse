; Inno Setup script — built by .github/workflows/release.yaml
; iscc /DAppVersion=0.1.0 installer\keypulse.iss

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif

[Setup]
AppId={{6F2B8C1E-4D7A-4B1F-9E3C-7A5D2C8B4E61}
AppName=keypulse
AppVersion={#AppVersion}
AppPublisher=jonathan-gtd
AppPublisherURL=https://github.com/jonathan-gtd/keypulse
AppSupportURL=https://github.com/jonathan-gtd/keypulse/issues
DefaultDirName={localappdata}\Programs\keypulse
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputDir=..\dist
OutputBaseFilename=keypulse-{#AppVersion}-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayName=keypulse
UninstallDisplayIcon={app}\keypulse.exe
LicenseFile=..\LICENSE

[Tasks]
Name: "autostart"; Description: "Start keypulse when I sign in"

[Files]
Source: "..\dist\keypulse.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\config.example.yaml"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{userprograms}\keypulse"; Filename: "{app}\keypulse.exe"
Name: "{userprograms}\keypulse configuration"; Filename: "{sys}\notepad.exe"; Parameters: """{userappdata}\keypulse\config.yaml"""

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "keypulse"; ValueData: """{app}\keypulse.exe"""; Flags: uninsdeletevalue; Tasks: autostart

[Run]
Filename: "{app}\keypulse.exe"; Description: "Start keypulse now"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM keypulse.exe"; Flags: runhidden; RunOnceId: "StopKeypulse"

[Code]
var
  OutputPage: TInputQueryWizardPage;
  OptionsPage: TInputOptionWizardPage;

function ConfigPath: String;
begin
  Result := ExpandConstant('{userappdata}\keypulse\config.yaml');
end;

procedure InitializeWizard;
begin
  OutputPage := CreateInputQueryPage(wpSelectTasks, 'Output', 'Where should keypulse send its statistics?',
    'udp://127.0.0.1:8094 is a Telegraf socket_listener on this computer. ' +
    'Use http(s)://... for Telegraf influxdb_listener or InfluxDB 2 (edit the token in config.yaml afterwards).');
  OutputPage.Add('Output URL:', False);
  OutputPage.Values[0] := 'udp://127.0.0.1:8094';

  OptionsPage := CreateInputOptionPage(OutputPage.ID, 'Options', 'What should keypulse record?',
    'You can change these later in ' + ConfigPath + '.', False, False);
  OptionsPage.Add('Break down activity by application');
  OptionsPage.Add('Cursor and click heatmap');
  OptionsPage.Add('Count shortcuts (CTRL+C, ALT+TAB, ...)');
  OptionsPage.Add('Count each key separately (keyboard heatmap)');
  OptionsPage.Values[0] := True;
  OptionsPage.Values[1] := True;
  OptionsPage.Values[2] := True;
  OptionsPage.Values[3] := False;
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
  // An existing configuration is kept on upgrade.
  Result := ((PageID = OutputPage.ID) or (PageID = OptionsPage.ID)) and FileExists(ConfigPath);
end;

function YesNo(B: Boolean): String;
begin
  if B then Result := 'true' else Result := 'false';
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  Code: Integer;
begin
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM keypulse.exe', '', SW_HIDE, ewWaitUntilTerminated, Code);
  Result := '';
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  Lines: TArrayOfString;
begin
  if (CurStep = ssPostInstall) and not FileExists(ConfigPath) then
  begin
    ForceDirectories(ExtractFileDir(ConfigPath));
    SetArrayLength(Lines, 15);
    Lines[0] := '# keypulse configuration. All settings: ' + ExpandConstant('{app}') + '\config.example.yaml';
    Lines[1] := 'interval: 60s';
    Lines[2] := 'per_app: ' + YesNo(OptionsPage.Values[0]);
    Lines[3] := 'per_key: ' + YesNo(OptionsPage.Values[3]);
    Lines[4] := 'shortcuts: ' + YesNo(OptionsPage.Values[2]);
    Lines[5] := 'clipboard: true';
    Lines[6] := 'session_lock: true';
    Lines[7] := 'heatmap:';
    Lines[8] := '  enabled: ' + YesNo(OptionsPage.Values[1]);
    Lines[9] := 'tray:';
    Lines[10] := '  enabled: true';
    Lines[11] := 'ui:';
    Lines[12] := '  addr: 127.0.0.1:8099';
    Lines[13] := 'output:';
    Lines[14] := '  url: "' + OutputPage.Values[0] + '"';
    SaveStringsToUTF8File(ConfigPath, Lines, False);
  end;
end;
