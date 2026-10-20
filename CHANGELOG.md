# Changelog

## [0.3.0] - 2026-10-20

Added
- **Notification-area icon**, with a menu to open the statistics or quit, so keypulse is never running invisibly.
- **Local status page** (`http://127.0.0.1:8099`): live totals since start, per-application table, where the data goes and what is being counted.
- **Shortcuts**: a counter per combination, such as `CTRL+C` or `ALT+TAB`. The combination is named; no key sequence is kept.
- **Key and click hold times** (total and count, so an average can be charted).
- **Double-clicks**.
- **Typing-speed peaks**: most keys in any one-second and ten-second window.
- **Foreground time per application**, counted even when nothing is pressed.
- **Idle seconds**, **application switches**, **clipboard-change count** (content never read) and **session lock/unlock**.
- New measurements `keypulse_session` and `keypulse_shortcuts`.

## [0.2.0] - 2026-10-20

- `cursor_m`: on-screen cursor travel in meters, from the physical size of the monitors — no DPI to configure.
- Per-monitor DPI awareness: heatmap cells and distances are right on scaled displays (125 %, 150 %…).
- While a game confines the cursor, no cursor distance or heatmap sample is recorded.

## [0.1.0] - 2026-10-20

First release.

- Key presses, clicks per button, wheel and mouse distance, read through the Raw Input API.
- Active time: seconds with at least one input.
- Per-application breakdown, from the foreground window.
- Optional per-key counts, for a keyboard heatmap.
- Cursor and click heatmap on a configurable grid.
- Line protocol output to Telegraf (UDP or HTTP) or InfluxDB 2.
- Below-normal priority and EcoQoS, so it stays on efficiency cores.
- Per-user installer that starts keypulse at sign-in.
