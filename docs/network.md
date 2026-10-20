# Per-application network traffic

keypulse can also report how many bytes each application sends and
receives — the one WhatPulse feature that needs more than a user program.

> **Verified on 2026-10-20** on an i9-14900KF / Windows 11: the per-process
> byte counts decode correctly from the kernel network provider. It ships in
> the normal keypulse binary and is off until you turn it on.

## Why there is a service, in one program

Windows does not expose per-process network traffic through any ordinary
counter. The only source is **Event Tracing for Windows**, and opening a
trace session is privileged.

A service, though, runs in session 0, where there is no desktop: it cannot
receive Raw Input and cannot see which window is in front. So no single
process can do both halves — that part is Windows, not a choice.

What *is* a choice is how many programs you end up with, and the answer is
one. `keypulse.exe` is the agent when you run it, and the network collector
when the service manager starts it. One download, one installer, one
configuration file, one icon.

## Turning it on

**From the tray icon:** click it and tick **Network statistics**. Windows
asks for consent once — registering a service needs it — and that is all.
Untick it to remove the service again.

**Or from a terminal**, as Administrator:

```powershell
keypulse.exe net-install     # register and start the service
keypulse.exe net-uninstall   # stop and remove it
keypulse.exe net             # run it in this console instead, Ctrl-C to stop
keypulse.exe net-check       # verify the Windows structure layouts (no admin)
```

The service takes its settings from your `config.yaml` at the moment you
enable it — the same output, host and interval as the agent — and keeps
them, because a service running as LocalSystem cannot read your profile.
Change the configuration and you will want to toggle it off and on again.

`net-check` is worth running first: it confirms every structure shared with
Windows has the size and offsets the C headers give, which is the
difference between a working trace and a crash, and it needs no rights at
all.

## What it reads

For each network event the kernel reports, exactly two numbers: the
**process id** and the **number of bytes**. Nothing else — no address, no
port, no hostname, no packet content. Those are added up per executable:

```
keypulse_net,host=pc,app=chrome.exe bytes_sent=182334i,bytes_recv=9281004i,bytes_total=9463338i,events=1420i <ts>
```

It writes wherever the agent writes, so Telegraf picks it up with the
`socket_listener` already there.

## If something goes wrong

The service logs to `%ProgramData%\keypulse\keypulse-net.log`. If a trace
session is ever left behind — after a hard kill, say — `logman stop
keypulse-net -ets` clears it, and the next start cleans up stale sessions
by itself.

## Charting it

```sql
SELECT t.app, sum(n.bytes_recv) AS down, sum(n.bytes_sent) AS up
FROM keypulse_net n JOIN keypulse_net_tag t ON t.tag_id = n.tag_id
WHERE $__timeFilter(n.time)
GROUP BY 1 ORDER BY 2 + 3 DESC LIMIT 15
```
