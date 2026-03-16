package bot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/igorvoltaic/keenetic-bot/router"
	tele "gopkg.in/telebot.v3"
)

type unrouteState struct {
	Rules    map[string][]string // outboundTag → entries (snapshot at command time)
	Outbound string              // selected rule
	Pending  string              // selected entry
}

var (
	btnUnrouteRule  = tele.InlineButton{Unique: "un_r"}
	btnUnrouteEntry = tele.InlineButton{Unique: "un_e"}
	btnUnrouteAct   = tele.InlineButton{Unique: "un_a"}
	btnUnrouteBack  = tele.InlineButton{Unique: "un_b"}
)

func (b *Bot) handleUnroute(c tele.Context) error {
	if b.cfg.Router.RoutingFile == "" || len(b.cfg.Router.RoutingOutbounds) == 0 {
		return c.Send("routing_file или routing_outbounds не настроены в конфиге")
	}

	rules := make(map[string][]string, len(b.cfg.Router.RoutingOutbounds))
	for _, tag := range b.cfg.Router.RoutingOutbounds {
		entries, err := router.ReadDomainEntries(b.cfg.Router.RoutingFile, tag)
		if err != nil {
			return c.Send(fmt.Sprintf("Ошибка чтения правила %s: %v", tag, err))
		}
		rules[tag] = entries
	}

	state := &unrouteState{Rules: rules}
	b.unrouteStates.Store(c.Sender().ID, state)
	return b.showUnrouteRules(c, state, false)
}

func (b *Bot) handleUnrouteRule(c tele.Context) error {
	v, ok := b.unrouteStates.Load(c.Sender().ID)
	if !ok {
		_ = c.Respond()
		return c.Edit("Сессия устарела, введи /unroute заново")
	}
	state := v.(*unrouteState)

	idx, err := strconv.Atoi(c.Callback().Data)
	if err != nil || idx < 0 || idx >= len(b.cfg.Router.RoutingOutbounds) {
		return c.Respond(&tele.CallbackResponse{Text: "Неверный выбор"})
	}
	state.Outbound = b.cfg.Router.RoutingOutbounds[idx]

	_ = c.Respond()
	return b.showUnrouteEntries(c, state)
}

func (b *Bot) handleUnrouteEntry(c tele.Context) error {
	v, ok := b.unrouteStates.Load(c.Sender().ID)
	if !ok {
		_ = c.Respond()
		return c.Edit("Сессия устарела, введи /unroute заново")
	}
	state := v.(*unrouteState)

	entries := state.Rules[state.Outbound]
	idx, err := strconv.Atoi(c.Callback().Data)
	if err != nil || idx < 0 || idx >= len(entries) {
		return c.Respond(&tele.CallbackResponse{Text: "Неверный выбор"})
	}
	state.Pending = entries[idx]

	_ = c.Respond()
	return b.showUnrouteAction(c, state)
}

func (b *Bot) handleUnrouteAct(c tele.Context) error {
	v, ok := b.unrouteStates.Load(c.Sender().ID)
	if !ok {
		_ = c.Respond()
		return c.Edit("Сессия устарела, введи /unroute заново")
	}
	state := v.(*unrouteState)

	if err := router.RemoveFromRoutingRule(b.cfg.Router.RoutingFile, state.Outbound, state.Pending); err != nil {
		_ = c.Respond()
		return c.Edit(fmt.Sprintf("Ошибка: %v", err))
	}

	b.unrouteStates.Delete(c.Sender().ID)
	_ = c.Respond()
	return c.Edit(fmt.Sprintf("✓ Удалено:\n%s\nиз %s", state.Pending, state.Outbound))
}

func (b *Bot) handleUnrouteBack(c tele.Context) error {
	v, ok := b.unrouteStates.Load(c.Sender().ID)
	if !ok {
		_ = c.Respond()
		return c.Edit("Сессия устарела, введи /unroute заново")
	}
	state := v.(*unrouteState)
	_ = c.Respond()

	switch c.Callback().Data {
	case "r":
		return b.showUnrouteRules(c, state, true)
	case "e":
		return b.showUnrouteEntries(c, state)
	default:
		return nil
	}
}

func (b *Bot) showUnrouteRules(c tele.Context, state *unrouteState, edit bool) error {
	var rows [][]tele.InlineButton
	for i, tag := range b.cfg.Router.RoutingOutbounds {
		rows = append(rows, []tele.InlineButton{{
			Unique: "un_r",
			Text:   fmt.Sprintf("%s (%d)", tag, len(state.Rules[tag])),
			Data:   strconv.Itoa(i),
		}})
	}
	menu := &tele.ReplyMarkup{InlineKeyboard: rows}
	if edit {
		return c.Edit("Выбери правило:", menu)
	}
	return c.Send("Выбери правило:", menu)
}

func (b *Bot) showUnrouteEntries(c tele.Context, state *unrouteState) error {
	entries := state.Rules[state.Outbound]
	if len(entries) == 0 {
		return c.Edit(state.Outbound + ": нет записей.")
	}

	var rows [][]tele.InlineButton
	for i, e := range entries {
		rows = append(rows, []tele.InlineButton{{
			Unique: "un_e",
			Text:   routingEntryLabel(e),
			Data:   strconv.Itoa(i),
		}})
	}
	rows = append(rows, []tele.InlineButton{{
		Unique: "un_b",
		Text:   "← Назад",
		Data:   "r",
	}})

	menu := &tele.ReplyMarkup{InlineKeyboard: rows}
	return c.Edit(fmt.Sprintf("%s (%d записей):\n\nВыбери запись:", state.Outbound, len(entries)), menu)
}

func (b *Bot) showUnrouteAction(c tele.Context, state *unrouteState) error {
	keyboard := [][]tele.InlineButton{
		{{Unique: "un_a", Text: "🗑 Удалить", Data: "del"}},
		{{Unique: "un_b", Text: "← Назад", Data: "e"}},
	}
	menu := &tele.ReplyMarkup{InlineKeyboard: keyboard}
	return c.Edit(fmt.Sprintf("%s:\n\n%s\n\nДействие:", state.Outbound, state.Pending), menu)
}

// routingEntryLabel returns a compact display label for a routing entry.
// "ext:geosite_v2fly.dat:youtube" → "geosite_v2fly:youtube"
func routingEntryLabel(entry string) string {
	s := strings.TrimPrefix(entry, "ext:")
	s = strings.ReplaceAll(s, ".dat:", ":")
	if len(s) > 40 {
		return s[:37] + "..."
	}
	return s
}
