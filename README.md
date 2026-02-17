# yucca

Simple Windows CLI to install and update the Helium browser.

## What it does

- Installs Helium from official GitHub releases
- Adds a Start Menu shortcut by default
- Can add a Desktop shortcut
- Checks for updates and installs newer versions
- Uninstalls Helium and cleans up shortcuts

## Build

```bash
go build -o yucca.exe ./cmd/yucca
```

Build with version from current git tag/commit:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

## Quick start

```bash
yucca install
yucca status
yucca update
yucca uninstall
```

## Commands

- `yucca install [--desktop-shortcut] [--force] [--quiet] [--json]`
- `yucca update [--check-only] [--force] [--quiet] [--json]`
- `yucca status [--json]`
- `yucca uninstall [--purge-config] [--json]`

## Notes

- Windows only (since other platforms have auto update support)
- State file: `%LOCALAPPDATA%\Yucca\state.json`
