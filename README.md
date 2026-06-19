# <img src="./assets/icon.png" width="48" height="48" valign="middle" /> Task Bar Trade Center

[![Patreon](https://img.shields.io/badge/Patreon-Donate-F96854?style=for-the-badge&logo=patreon&logoColor=white)](https://www.patreon.com/16264399/join)

Task Bar Trade Center is a Windows tray utility for TaskBarHero, created by nidea1. It watches the item under the in-game cursor, fetches Steam Community Market pricing, and draws a small price HUD near the game tooltip.

## Features

- Runs from the Windows notification area.
- Waits in the background until `TaskBarHero.exe` is launched.
- Shows `Waiting for TaskBarHero` in the tray tooltip while waiting for the game.
- Shows a compact market price overlay for marketable items (supports **Detail** and **Compact** modes).
- Opens the active item's Steam Market listing with middle mouse button while the price overlay is visible.
- Uses `assets/icon.png` as the Windows application and tray icon.
- Embeds `items.json` into the executable, so release builds are single-file.
- Persists user preferences and price cache between launches.
- Automatic log rotation (automatically prunes debug logs if they exceed 5MB).
- Tray menu actions:
  - `Refresh cached prices`
  - `Clear cache`
  - `Switch to Compact/Detail mode`
  - `Check for updates...`
  - `Exit`

## UI Modes

The pricing HUD overlay can be toggled between two modes from the tray context menu:

- **Detail Mode (Default):** Shows comprehensive statistics, including suggested pricing, weekly averages, daily volume, trend percentages, spreads, buy/sell orders, and a deal assessment tag (e.g. "Undervalued", "Overvalued").
- **Compact Mode:** A minimal HUD layout focused purely on critical metrics (Suggested, Last Sold, Lowest Sell, Highest Buy, Weekly Average, Daily Sales) to minimize screen footprint.

## Screenshots

| Detail Mode HUD | Compact Mode HUD |
| :---: | :---: |
| ![Detail Mode HUD](./assets/detailed.png) | ![Compact Mode HUD](./assets/compact.png) |

### System Tray Menu

![System Tray Menu](./assets/tray-menu.png)

## Requirements

- Windows amd64.
- Go 1.26.3 or newer for local builds.
- Permission to read the game process memory. If attach fails, run the app as Administrator.

## Development

```powershell
go test ./...
go build -o .tmp/tbtc-dev.exe .
```

## User data

Release builds write logs, settings, and cache under the user's local app data folder:

```text
%LOCALAPPDATA%\Task Bar Trade Center\config\settings.json      - Persists user preferences (e.g., overlay mode)
%LOCALAPPDATA%\Task Bar Trade Center\logs\tbtc.log - Debug logs (automatically capped at 5MB)
%LOCALAPPDATA%\Task Bar Trade Center\cache\price-cache.json    - Persisted price cache
```

If a user reports a bug, ask for the log file. The cache and refresh menu actions are disabled until the app attaches to `TaskBarHero.exe`.

## Build

Console build for debugging:

```powershell
go build -o .tmp/tbtc-dev.exe .
```

Release-style GUI build:

```powershell
New-Item -ItemType Directory -Force -Path dist
go build -trimpath -ldflags="-s -w -H=windowsgui" -o dist/tbtc.exe .
```

The workflow uploads the Windows `.exe` and a SHA-256 checksum file.

## Antivirus & Security Warnings

Because this utility attaches to the `TaskBarHero.exe` process and reads its memory space (`ReadProcessMemory`) to dynamically locate tooltips and active item IDs, some security software may flag the executable as a heuristic or generic detection (false-positive). 

- **Permissions:** If the application fails to attach, make sure to **Run as Administrator**.
- **VirusTotal Scan:** For transparency, you can view the official VirusTotal analysis of compiled releases here:
  - [VirusTotal Analysis (Release v0.1.0)](https://www.virustotal.com/gui/file/a02f86e36b00630c7cb1dc08a19cb747b08b0a5c63bf2e8f337f22702012e7c2/detection)

If your antivirus flags this utility, you may need to add it to your exclusion list.

## Support & Donations

If you find this tool helpful and want to support its development, feel free to support me on Patreon!

[![Patreon](https://img.shields.io/badge/Patreon-Donate-F96854?style=for-the-badge&logo=patreon&logoColor=white)](https://www.patreon.com/16264399/join)

## Acknowledgements & Credits

- Special thanks to the creators of [Allyans3/steam-market-api-v2](https://github.com/Allyans3/steam-market-api-v2) and other Steam Community Market parser projects for referencing their API formats and JSON structures.
- Huge thanks to the contributors of [TaskBarHero Wiki](https://taskbarhero.wiki/) for providing the database mapping structure for `items.json`.
