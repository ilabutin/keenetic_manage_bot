package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/igorvoltaic/keenetic-bot/bot"
	"github.com/igorvoltaic/keenetic-bot/config"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	b, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("bot init: %v", err)
	}

	log.Println("Bot started")
	b.Start()
}
