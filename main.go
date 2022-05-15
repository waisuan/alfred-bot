package main

import (
	"github.com/joho/godotenv"
	"os"
)

func main() {
	_ = godotenv.Load(".env")

	token := os.Getenv("SLACK_AUTH_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	b := NewBot(token, appToken)
	b.Start()
}
