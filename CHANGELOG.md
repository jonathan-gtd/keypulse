# Changelog

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
