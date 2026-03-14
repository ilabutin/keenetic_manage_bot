# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Telegram bot for managing a Keenetic home router running xkeen (Xray-based VPN). The bot runs **directly on the router** via Entware and controls it via local shell commands (`os/exec`). No SSH layer — commands execute in-process.

## Commands

```bash
# Development build (current platform)
make build

# Cross-compile for router (MIPS big-endian, Linux 4.9)
make build-router

# Run locally for testing (requires config.yaml)
make run

# Fetch/tidy dependencies
make tidy
go mod tidy
```

## Architecture

```
main.go          — CLI flag parsing, wires config → bot
config/          — YAML config loader (token, allowed_user_ids, xkeen_init_script)
bot/
  bot.go         — tele.Bot init, handler registration, global auth middleware
  handlers.go    — /start /reboot /xkeen /clients command implementations
  middleware.go  — allowOnly() middleware: rejects non-whitelisted Telegram user IDs
router/
  router.go      — Reboot() and XkeenCmd() via os/exec
  clients.go     — ConnectedClients() via `ndmc -c "show ip hotspot"` (returns JSON)
```

**Telegram library:** `gopkg.in/telebot.v3`

**Security model:** All commands are blocked by `allowOnly` middleware; only `telegram.allowed_user_ids` from config can interact with the bot.

**Router commands used:**
- `reboot` — system reboot
- `<xkeen_init_script> <start|stop|restart|status>` — default: `/opt/etc/init.d/S99xkeen`
- `ndmc -c "show ip hotspot"` — list active DHCP clients (returns JSON with a `"host"` array)

## Config

Copy `config.example.yaml` → `config.yaml` and fill in the bot token and your Telegram user ID.
Get your Telegram ID from `@userinfobot`.

## Router

`uname -a`: `Linux ILN-Main-Router 4.9-ndm-5 MIPS GNU/Linux` — несмотря на "mips" в uname, user-space (Entware) little-endian: `opkg print-architecture` → `mipsel-3.4`. Сборка: `GOARCH=mipsle GOMIPS=softfloat`.

Place the binary on the router's Entware partition: `/opt/sbin/keenetic-bot`.
