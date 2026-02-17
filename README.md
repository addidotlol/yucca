# yucca

Simple Windows CLI to install and auto-update the [Helium browser](https://helium.computer).

## [I DONT GIVE A 🐕 ABOUT THE 🐕ING CODE!](https://github.com/addidotlol/yucca/releases/latest/download/yucca.exe)

## TL;DR

- Download `yucca` above.
- Place somewhere nice and safe
- Open terminal where it is
- `./yucca.exe install`
- Run Helium, now you have auto updates

## What it does

- Installs Helium from official GitHub releases
- Adds a Start Menu shortcut by default
- Adds a Desktop shortcut by default
- Launches Helium through Yucca with a pre-launch update check
- Automatically applies updates when launching through the shortcut flow
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
yucca launch
yucca status
yucca update
yucca uninstall
```

## Commands

- `yucca install [--desktop-shortcut] [--no-desktop-shortcut] [--force] [--quiet] [--json]`
- `yucca launch [--verbose]`
- `yucca update [--check-only] [--force] [--quiet] [--json]`
- `yucca status [--json]`
- `yucca uninstall [--purge-config] [--json]`

## Notes

- Windows only (since other platforms have auto update support)
- State file: `%LOCALAPPDATA%\Yucca\state.json`
