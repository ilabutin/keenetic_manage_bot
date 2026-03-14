# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Telegram bot for managing a Keenetic home router running xkeen (Xray-based VPN). The bot runs **directly on the router** via Entware and controls it via local shell commands (`os/exec`). No SSH layer — commands execute in-process.

## Commands

```bash
make build          # dev build (current platform)
make build-router   # cross-compile for router (mipsle)
make deploy         # build + stop bot + upload binary + upload init script + start bot
make run            # run locally (requires config.yaml)
make tidy           # go mod tidy
```

`deploy` connects to `ROUTER_HOST` (default `172.16.0.1`) as `ROUTER_USER` (default `root`).

## Architecture

```
main.go              — CLI flag parsing (-config), wires config → bot
config/config.go     — YAML config loader
bot/
  bot.go             — tele.Bot init, OnError handler, handler registration
  handlers.go        — command handlers + formatBytes helper
  middleware.go      — allowOnly(): rejects non-whitelisted Telegram user IDs
router/
  router.go          — run() helper (exec + ANSI strip), Reboot(), XkeenCmd()
  clients.go         — ConnectedClients() + parseHotspot() for ndmc text output
  sysinfo.go         — SystemInfo(), GeoUpdateTime(), ProcessUptime()
init.d/
  S99keenetic-bot    — Entware init script (uses rc.func)
```

**Telegram library:** `gopkg.in/telebot.v3`

**Security:** global `allowOnly` middleware — only `telegram.allowed_user_ids` from config can use the bot.

## Bot commands

| Command | Description |
|---|---|
| `/sysinfo` | Uptime, load, RAM, xray uptime, xkeen/geo file dates |
| `/clients` | Active devices sorted by network (Home first), with rx/tx traffic |
| `/xkeen <start\|stop\|restart\|status>` | Calls `/opt/sbin/xkeen -<action>` |
| `/reboot` | Reboots the router |

## Router commands used

- `/bin/ndmc -c "show ip hotspot"` — active DHCP clients (text format, not JSON)
- `/opt/sbin/xkeen -<action>` — xkeen management
- `reboot` — system reboot
- `/proc/uptime`, `/proc/loadavg`, `/proc/meminfo` — read directly in Go
- `/proc/<pid>/stat` — xray process uptime

## Config

Copy `config.example.yaml` → `config.yaml`. Key fields:

```yaml
telegram:
  token: "..."
  allowed_user_ids: [123456789]
router:
  xkeen_path: "/opt/sbin/xkeen"       # default
  xkeen_dat_dir: "/opt/etc/xray/dat"  # default
```

## Router

- Model: Keenetic Giga, `uname -m` = `mips`, but Entware is **little-endian**: `opkg print-architecture` → `mipsel-3.4`
- Build: `GOARCH=mipsle GOMIPS=softfloat`
- Binary: `/opt/sbin/keenetic-bot`, config: `/opt/etc/keenetic-bot.yaml`
- Geo dat files: `/opt/etc/xray/dat/` (geoip_v2fly, geosite_v2fly, geoip_refilter, geosite_refilter, zkeen, zkeenip)
- Telegram (`api.telegram.org`) is reachable directly — no proxy needed

## ndmc output format

`ndmc -c "show ip hotspot"` returns indented text (not JSON). Each device block starts with `host:`, contains `mac/ip/hostname/name/mws-backhaul`, then an `interface:` sub-block (id/name/description), then `active: yes/no`, `rxbytes`, `txbytes`. Symlinks in dat dir are skipped when reading mod times.
