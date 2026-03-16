package bot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/igorvoltaic/keenetic-bot/router"
	tele "gopkg.in/telebot.v3"
)

type routeState struct {
	Domain     string
	Categories []router.GeoCategory
	Pending    string // entry to add (set after category/domain selection)
}

// Button singletons — Unique is the handler endpoint in telebot v3.
var (
	btnRouteSel = tele.InlineButton{Unique: "rt_sel"}
	btnRouteOut = tele.InlineButton{Unique: "rt_out"}
)

func (b *Bot) handleRoute(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("Использование: /route <домен>")
	}

	if b.cfg.Router.GeoDataTool == "" || b.cfg.Router.RoutingFile == "" {
		return c.Send("Ошибка: geodat_tool и routing_file не настроены в конфиге")
	}
	if len(b.cfg.Router.RoutingOutbounds) == 0 {
		return c.Send("Ошибка: routing_outbounds не настроены в конфиге")
	}

	domain := strings.ToLower(strings.TrimSpace(args[0]))

	cats, err := router.LookupDomain(b.cfg.Router.GeoDataTool, b.cfg.Router.XkeenDatDir, domain)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка поиска: %v", err))
	}

	b.routeStates.Store(c.Sender().ID, &routeState{
		Domain:     domain,
		Categories: cats,
	})

	keyboard := make([][]tele.InlineButton, 0, len(cats)+1)
	for i, cat := range cats {
		keyboard = append(keyboard, []tele.InlineButton{{
			Unique: "rt_sel",
			Text:   cat.Label(),
			Data:   strconv.Itoa(i),
		}})
	}
	keyboard = append(keyboard, []tele.InlineButton{{
		Unique: "rt_sel",
		Text:   "➕ домен: " + domain,
		Data:   "d",
	}})

	var msg strings.Builder
	msg.WriteString("Домен: " + domain)
	if len(cats) > 0 {
		msg.WriteString(fmt.Sprintf("\nКатегорий найдено: %d\n\nЧто добавить?", len(cats)))
	} else {
		msg.WriteString("\nКатегории не найдены.")
	}

	return c.Send(msg.String(), &tele.ReplyMarkup{InlineKeyboard: keyboard})
}

func (b *Bot) handleRouteSel(c tele.Context) error {
	v, ok := b.routeStates.Load(c.Sender().ID)
	if !ok {
		_ = c.Respond()
		return c.Edit("Сессия устарела, введи /route заново")
	}
	state := v.(*routeState)

	data := c.Callback().Data
	var entry string
	if data == "d" {
		entry = state.Domain
	} else {
		idx, err := strconv.Atoi(data)
		if err != nil || idx < 0 || idx >= len(state.Categories) {
			return c.Respond(&tele.CallbackResponse{Text: "Неверный выбор"})
		}
		entry = state.Categories[idx].Entry()
	}
	state.Pending = entry

	var row []tele.InlineButton
	for i, out := range b.cfg.Router.RoutingOutbounds {
		row = append(row, tele.InlineButton{
			Unique: "rt_out",
			Text:   out,
			Data:   strconv.Itoa(i),
		})
	}

	_ = c.Respond()
	return c.Edit(
		"Добавить: "+entry+"\n\nВыбери правило:",
		&tele.ReplyMarkup{InlineKeyboard: [][]tele.InlineButton{row}},
	)
}

func (b *Bot) handleRouteOut(c tele.Context) error {
	v, ok := b.routeStates.Load(c.Sender().ID)
	if !ok {
		_ = c.Respond()
		return c.Edit("Сессия устарела, введи /route заново")
	}
	state := v.(*routeState)

	idx, err := strconv.Atoi(c.Callback().Data)
	if err != nil || idx < 0 || idx >= len(b.cfg.Router.RoutingOutbounds) {
		return c.Respond(&tele.CallbackResponse{Text: "Неверный выбор"})
	}
	outbound := b.cfg.Router.RoutingOutbounds[idx]

	if err := router.AddToRoutingRule(b.cfg.Router.RoutingFile, outbound, state.Pending); err != nil {
		_ = c.Respond()
		return c.Edit(fmt.Sprintf("Ошибка: %v", err))
	}

	b.routeStates.Delete(c.Sender().ID)
	_ = c.Respond()
	return c.Edit(fmt.Sprintf("✓ Добавлено:\n%s\n→ %s", state.Pending, outbound))
}
