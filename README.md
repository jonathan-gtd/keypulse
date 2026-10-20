<div align="center">

# keypulse

### Keyboard and mouse statistics for your own time-series database

Keys, clicks, scrolling, mouse travel, per-application breakdown and heatmaps —
sent to Telegraf or InfluxDB, charted in Grafana. Like WhatPulse, without the cloud.

[![Release](https://img.shields.io/github/v/release/jonathan-gtd/keypulse?color=41BDF5)](https://github.com/jonathan-gtd/keypulse/releases/latest) [![CI](https://img.shields.io/github/actions/workflow/status/jonathan-gtd/keypulse/ci.yaml?branch=main&label=tests)](https://github.com/jonathan-gtd/keypulse/actions/workflows/ci.yaml) [![License](https://img.shields.io/github/license/jonathan-gtd/keypulse?color=lightgrey)](LICENSE)

[![lang en](https://img.shields.io/badge/lang-en-41BDF5)](README.md) [![lang fr](https://img.shields.io/badge/lang-fr-lightgrey)](README.fr.md)

</div>

---

[WhatPulse](https://whatpulse.org) and [MouseTracks](https://github.com/huntfx/MouseTracks) show how you use your keyboard and mouse, but keep the data in their own app or cloud. keypulse is a small Windows agent that does the counting and hands the numbers to the stack you already run — Telegraf, InfluxDB, TimescaleDB, Grafana.

- ⌨️ **Keys, clicks, wheel, mouse distance and active time**, every minute — cursor travel in real meters, no DPI to enter.
- 🪟 **Per application** — how much you type in your editor, click in your game.
- 🔥 **Heatmaps** — where the cursor goes and where you click, and (opt-in) which keys you use.
- 🎮 **Built not to cost a frame** — no input hook, below-normal priority, efficiency cores. [Details](#performance)
- 🔒 **Counts only** — never what you type. [Details](#privacy)

---

## Install

**1. Download** `keypulse-<version>-setup.exe` from the [latest release](https://github.com/jonathan-gtd/keypulse/releases/latest) and run it. It installs for your user only (no administrator rights) and starts keypulse when you sign in.

> The installer is not code-signed yet: Windows SmartScreen may ask you to confirm (*More info → Run anyway*). Checksums are published with each release.

**2. Give it somewhere to send the numbers.** The default is a Telegraf `socket_listener` on the same computer:

```toml
[[inputs.socket_listener]]
  service_address = "udp://127.0.0.1:8094"
  data_format = "influx"
```

From there, any Telegraf output works: InfluxDB, PostgreSQL/TimescaleDB, MQTT, Prometheus… See [docs/telegraf.md](docs/telegraf.md) for a TimescaleDB example. keypulse can also post to InfluxDB 2 directly (`output.url` in the configuration).

---

## Configuration

`%APPDATA%\keypulse\config.yaml`, written by the installer. Every setting is optional — see [config.example.yaml](config.example.yaml).

| Setting | Default | |
|---|---|---|
| `interval` | `60s` | How often counters are sent, aligned on the clock |
| `per_app` | `true` | Break activity down by foreground application |
| `per_key` | `false` | Count every key separately |
| `heatmap.enabled` | `true` | Cursor and click positions |
| `heatmap.cell_size` | `40` | Heatmap grid, in pixels |
| `mouse_dpi` | `0` | Your mouse DPI, to also get the distance the mouse travels on the desk (`mouse_m`) |
| `output.url` | `udp://127.0.0.1:8094` | `udp://` (Telegraf), `http(s)://` (Telegraf listener, InfluxDB 2) |
| `output.token` | | InfluxDB token |
| `host` | computer name | Value of the `host` tag |

Restart keypulse after a change: end it in Task Manager, then start it again from the Start menu.

---

## Data

Line protocol, one batch per interval, nothing sent when nothing happened:

```
keypulse,host=pc,app=code.exe keys=412i,clicks=37i,clicks_left=31i,clicks_right=6i,clicks_middle=0i,clicks_x1=0i,clicks_x2=0i,wheel_up=24,wheel_down=31,wheel_left=0,wheel_right=0,mouse_distance=48211.5,cursor_m=11.2731,active_s=58i 1760000000000000000
keypulse_keys,host=pc,key=SPACE count=61i 1760000000000000000
keypulse_heatmap,host=pc,kind=click,x=31,y=17 count=4i 1760000000000000000
```

| Measurement | Tags | Fields |
|---|---|---|
| `keypulse` | `host`, `app` | `keys`, `clicks`, `clicks_<button>`, `wheel_up/down/left/right` (notches), `mouse_distance` (sensor counts), `cursor_m` (on-screen cursor travel, meters), `mouse_m` (if `mouse_dpi` is set), `active_s` |
| `keypulse_keys` | `host`, `key` | `count` — only with `per_key` |
| `keypulse_heatmap` | `host`, `kind` (`move`/`click`), `x`, `y` | `count` — cells from the top-left corner of the desktop |

`cursor_m` is measured like WhatPulse does it: the distance the cursor covers on screen, converted to meters with the physical size Windows reads from each monitor (EDID). Mouse DPI cannot be read by software, so this needs no setting. Games that capture the mouse confine the cursor; keypulse detects it and adds neither cursor distance nor heatmap samples while it lasts — `mouse_distance` keeps counting.

Key names follow Windows virtual keys (`A`, `SPACE`, `LCTRL`, `NUMPAD_ENTER`, `OEM_1`…), so letters follow your keyboard layout. A held key counts once, not once per auto-repeat.

---

## Performance

keypulse is meant to run while you play.

- **Raw Input, not a hook.** Low-level keyboard and mouse hooks sit in the path between your devices and every application; a slow hook delays input. keypulse registers for [Raw Input](https://learn.microsoft.com/windows/win32/inputdev/raw-input) instead: Windows hands it a copy of each event, and the game never waits for it.
- **Below-normal priority and [EcoQoS](https://devblogs.microsoft.com/performance-diagnostics/introducing-ecoqos/)** — on hybrid CPUs, Windows keeps it on efficiency cores.
- **No polling.** The foreground application comes from a Windows event; the cursor position is read at most 10 times a second, and only after the mouse moved.
- **No allocation per event**, one small network packet per interval.

Measured on an i9-14900KF over 30 seconds of ordinary mouse use, start-up included: 0.6 % of one core (0.02 % of the whole CPU) and 48 MB of memory.

---

## Privacy

keypulse counts; it does not log. Totals never say which keys were pressed. `per_key` records how many times each key was pressed per interval — never their order — but on short intervals that can still hint at what was typed, which is why it is off by default. Everything goes only to the address you configure.

---

## Build

Go 1.24 or later:

```
GOOS=windows GOARCH=amd64 go build -ldflags "-H=windowsgui" -o keypulse.exe ./cmd/keypulse
```

`go test ./...` runs on any platform. Releases are built by GitHub Actions from a `v*` tag, installer included ([Inno Setup](https://jrsoftware.org/isinfo.php)).

## License

[MIT](LICENSE)
