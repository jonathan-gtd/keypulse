# Changelog

## [0.6.0] - 2026-10-20

Removed
- **Per-application network statistics**, and with them the whole ETW/service machinery. The total per-machine network use was already covered by the hardware monitoring (Telegraf `inputs.net`), and the per-application breakdown was not worth a permanent privileged service. keypulse is a plain user program again, with no sub-commands and nothing running as a service.

## [0.5.0] - 2026-10-20

Changed
- **One program instead of two.** The network collector is no longer a separate `keypulse-net.exe`: it is the same `keypulse.exe`, run as a service by the service manager. One download, one installer, one configuration file, one icon. Windows still needs two *processes* — a service has no desktop and cannot read raw input — but that is now an implementation detail rather than a second application to install.

Added
- **Network statistics** reporting bytes sent and received per application, from the kernel network provider through a minimal Event Tracing consumer written for the purpose. Only the process id and the byte count are read from each event — never an address, a port or any packet content. See [docs/network.md](docs/network.md).
- A **Network statistics** tick in the tray menu that turns it on, asking Windows for consent once instead of needing a command line.
- Sub-commands `net`, `net-install`, `net-uninstall` and `net-check`; the last verifies the Windows structure layouts and needs no rights.
- The status page says whether network statistics are on.
- Sub-commands print to the console they were launched from, although keypulse is a GUI program.

Note: off until you turn it on (the trace session needs administrator rights once). Verified working on real hardware on 2026-10-20.

## [0.4.0] - 2026-10-20

Added
- **Key categories**: presses split into letters, digits, punctuation, navigation, function, modifier, numpad and media.
- **Hand balance**: how much of your typing each half of the keyboard takes. Read from the physical scan code, so it is the same whatever the layout.
- **Typing rhythm**: a histogram of the gap between two presses (`<50ms` … `>1s`). The gaps are counted; the keys they fell between are not.
- **Peak cursor speed**, in metres per second.
- **Pointer-held seconds**: time with the pointer confined to part of the screen, which is what a game does when it takes the mouse — a decent measure of time spent in one.
- The status page shows the hand split, the peak cursor speed and the pointer-held time.
- New measurements `keypulse_keycat` and `keypulse_rhythm`; `keypulse_session` gains `hand_left`, `hand_right`, `captured_s` and `cursor_peak_mps`.

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
