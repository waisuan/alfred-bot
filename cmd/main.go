package main

import (
	"alfred-bot/cmd/bot"
	"alfred-bot/config"
	"os"
)

func main() {
	config.BootstrapEnv(false)

	token := os.Getenv("SLACK_AUTH_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	b := bot.New(token, appToken)
	b.Start()
}
