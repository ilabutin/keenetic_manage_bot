package bot

import (
	"fmt"
	"log"
	"time"

	"github.com/igorvoltaic/keenetic-bot/config"
	tele "gopkg.in/telebot.v3"
)

type Bot struct {
	cfg    *config.Config
	telebot *tele.Bot
}

func New(cfg *config.Config) (*Bot, error) {
	pref := tele.Settings{
		Token:  cfg.Telegram.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		OnError: func(err error, c tele.Context) {
			log.Printf("handler error: %v", err)
			if c != nil {
				_ = c.Send(fmt.Sprintf("Ошибка: %v", err))
			}
		},
	}
	tb, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}
	b := &Bot{cfg: cfg, telebot: tb}
	b.registerHandlers()
	return b, nil
}

func (b *Bot) Start() {
	b.telebot.Start()
}

func (b *Bot) registerHandlers() {
	guard := allowOnly(b.cfg.Telegram.AllowedUserIDs)
	b.telebot.Use(guard)

	b.telebot.Handle("/start", b.handleStart)
	b.telebot.Handle("/help", b.handleStart)
	b.telebot.Handle("/sysinfo", b.handleSysInfo)
	b.telebot.Handle("/clients", b.handleClients)
	b.telebot.Handle("/xkeen", b.handleXkeen)
	b.telebot.Handle("/reboot", b.handleReboot)
}
