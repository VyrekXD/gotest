package config

import (
	"log"
	"os"

	"github.com/disgoorg/disgo/gateway"
	"github.com/joho/godotenv"
)

var (
	Token string
	Intents  gateway.ConfigOpt = gateway.WithIntents(
		gateway.IntentGuilds,
		gateway.IntentGuildEmojisAndStickers,
		gateway.IntentGuildMessages,
		gateway.IntentGuildMessageReactions,
		gateway.IntentDirectMessages,
		gateway.IntentMessageContent,
	)
)

func init() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Panicf("Error when loading .env, %v", err)
	}

	Token = os.Getenv("TOKEN")
}