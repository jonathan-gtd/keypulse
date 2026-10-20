# Per-application network traffic

`keypulse-net` reports how many bytes each application sends and receives —
the one WhatPulse feature keypulse itself cannot provide.

> **Status: not yet released.** The code is on `main` and builds, but it has
> not been run on real hardware yet, so there is no release binary. Build it
> yourself if you want to try it, and please report what you find.

## Why it is a separate program

Windows does not expose per-process network traffic through any ordinary
counter. The only source is **Event Tracing for Windows**, and opening a
trace session is a privileged operation.

keypulse itself is deliberately a plain user program: no administrator
rights, no service, nothing in the system. Rather than give the whole agent
those rights, the network part is a second, small program that runs as a
service, and everything else stays as it was.

## What it reads

For each network event the kernel reports, it reads exactly two numbers:
the **process id** and the **number of bytes**. Nothing else — no address,
no port, no hostname, no packet content. Those are added up per executable
and sent once a minute:

```
keypulse_net,host=pc,app=chrome.exe bytes_sent=182334i,bytes_recv=9281004i,bytes_total=9463338i,events=1420i <ts>
```

It writes to the same place keypulse does, so Telegraf picks it up with the
`socket_listener` you already have.

## Install

```powershell
# from an Administrator PowerShell
.\keypulse-net.exe install -output udp://127.0.0.1:8094 -host mars
.\keypulse-net.exe uninstall     # to remove it again
```

`install` registers a service named `keypulse-net` that starts with Windows,
and starts it immediately. Options:

| Flag | Default | |
|---|---|---|
| `-output` | `udp://127.0.0.1:8094` | Same sinks as keypulse (`udp://`, `http(s)://`) |
| `-host` | computer name | Value of the `host` tag |
| `-interval` | `60s` | How often to report |
| `-measurement` | `keypulse_net` | Measurement name |
| `-log` | `%ProgramData%\keypulse\keypulse-net.log` | Log file |

Run it without a sub-command to try it in a console first (still as
administrator); Ctrl-C stops it and closes the trace session cleanly.

## Removing it

`.\keypulse-net.exe uninstall` stops and deletes the service. If a trace
session is ever left behind — after a hard kill, say — `logman stop
keypulse-net -ets` clears it, and the next start cleans up stale sessions by
itself.

## Charting it

```sql
SELECT t.app, sum(n.bytes_recv) AS down, sum(n.bytes_sent) AS up
FROM keypulse_net n JOIN keypulse_net_tag t ON t.tag_id = n.tag_id
WHERE $__timeFilter(n.time)
GROUP BY 1 ORDER BY 2 + 3 DESC LIMIT 15
```
