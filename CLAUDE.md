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
  clients.go         — ConnectedClients() + parseHotspot() for ndmc text output; WireguardPeers() + parseWireguardInterface()
  sysinfo.go         — SystemInfo(), GeoUpdateTime(), ProcessUptime()
  geodat.go          — LookupDomain(): runs xk-geodat lookup across all geosite dat files
  routing.go         — AddToRoutingRule(), RemoveFromRoutingRule(), ReadDomainEntries(); JSONC-aware text manipulation
init.d/
  S99keenetic-bot    — Entware init script (uses rc.func)
```

**Telegram library:** `gopkg.in/telebot.v3`

**Security:** global `allowOnly` middleware — only `telegram.allowed_user_ids` from config can use the bot.

**Reply keyboard:** `mainMenu` (defined in `handlers.go`) is a persistent ResizeKeyboard sent on `/start`/`/help`. Rows: `/sysinfo`+`/clients`; `/xkeen status`+`/xkeen restart`; `/unroute`+`/reboot`. No button for `/route` (requires domain argument).

## Bot commands

| Command | Description |
|---|---|
| `/sysinfo` | Uptime, load, RAM, xray uptime, xkeen/geo file dates |
| `/clients` | Active devices sorted by network (Home first), with rx/tx traffic; WireGuard peers (online/offline) if `wireguard_iface` is configured |
| `/xkeen <start\|stop\|restart\|status>` | Calls `/opt/sbin/xkeen -<action>` |
| `/route <domain>` | Search domain in geosite dat files; show matching categories + inline buttons to pick category (or raw domain) and target outbound rule |
| `/unroute` | Browse and remove entries from routing rules; two-step inline keyboard: pick rule → pick entry → [🗑 Удалить] |
| `/reboot` | Reboots the router |

## Router commands used

- `/bin/ndmc -c "show ip hotspot"` — active DHCP clients (text format, not JSON)
- `/bin/ndmc -c "show interface <iface>"` — WireGuard interface + peer status
- `/opt/etc/xkeen-ui/bin/xk-geodat lookup --kind geosite --path <file> --value <domain>` — returns JSON with `matches[].tag`
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
  wireguard_iface: "Wireguard0"       # optional; omit to hide WireGuard section in /clients
  geodat_tool: "/opt/etc/xkeen-ui/bin/xk-geodat"
  routing_file: "/opt/etc/xray/configs/05_routing.json"
  routing_outbounds: ["vless-reality", "vless-reality-fi"]
```

## Router

- Model: Keenetic Giga, `uname -m` = `mips`, but Entware is **little-endian**: `opkg print-architecture` → `mipsel-3.4`
- Build: `GOARCH=mipsle GOMIPS=softfloat`
- Binary: `/opt/sbin/keenetic-bot`, config: `/opt/etc/keenetic-bot.yaml`
- Geo dat files: `/opt/etc/xray/dat/` (geoip_v2fly, geosite_v2fly, geoip_refilter, geosite_refilter, zkeen, zkeenip)
- Telegram (`api.telegram.org`) is reachable directly — no proxy needed

## ndmc output format

`ndmc -c "show ip hotspot"` returns indented text (not JSON). Each device block starts with `host:`, contains `mac/ip/hostname/name/mws-backhaul`, then an `interface:` sub-block (id/name/description), then `active: yes/no`, `rxbytes`, `txbytes`. Symlinks in dat dir are skipped when reading mod times.

`ndmc -c "show interface Wireguard0"` returns indented text. After interface-level fields, each `peer:` block contains `description`, `online: yes/no`, `rxbytes`, `txbytes`, `remote-endpoint-address`. The `public-key:` value appears on its own continuation line (no colon). Peer blocks end at `summary:`.

## routing.json format

`/opt/etc/xray/configs/05_routing.json` is JSONC (JSON with `//` comments). Domain entries are strings like `"ext:geosite_v2fly.dat:youtube"` (for categories) or plain `"domain.com"`. The file is modified in-place using text manipulation that preserves comments — do not use `encoding/json` for write-back. The JSONC scanner in `router/routing.go` handles strings, `//` line comments, and `/* */` block comments. The backward scan in `findEnclosingBrace` is simplified (no string handling) and relies on routing.json rule values not containing `{`/`}`. **Critical:** `findNextOutside` checks for substring match BEFORE skipping strings — search strings like `"outboundTag"` and `"domain"` start with `"`, so the match check must come before the string-skip switch.

## /route and /unroute flow

`/route`: two-step inline keyboard: (1) pick category (e.g. `youtube (v2fly)` → `ext:geosite_v2fly.dat:youtube`) or "add domain directly"; (2) pick target outbound. State in `bot.routeStates` (sync.Map, int64→*routeState), cleared after addition.

`/unroute`: three-step inline keyboard with back navigation: (1) pick rule (shows entry count); (2) pick entry from list; (3) pick action [🗑 Удалить]. Entries snapshot is taken at command time and stored in `bot.unrouteStates` (sync.Map, int64→*unrouteState). `removeDomainEntry` reconstructs the domain array (comments inside arrays are not present in practice). Entry labels strip `ext:` prefix and `.dat:` infix for brevity.
