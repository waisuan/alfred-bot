package main

import (
	"alfred-bot/cmd/bot"
	"github.com/joho/godotenv"
	"os"
)

func main() {
	_ = godotenv.Load(".env")

	token := os.Getenv("SLACK_AUTH_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	b := bot.New(token, appToken)
	b.Start()
}
