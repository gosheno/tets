package chatbot

import (
	"tg-getgems-bot/botutils"

	"github.com/go-redis/redis/v8"
	"gopkg.in/telebot.v3"
)

// Bot defines the interface for chat bot modules
// You can expand this interface with more methods as needed.

// SimpleBot is a basic implementation of the Bot interface
type SimpleBot struct {
	Name        string
	RedisClient *redis.Client
}

// NewSimpleBot creates a new SimpleBot instance
func NewSimpleBot(name string, redisClient *redis.Client) *SimpleBot {
	return &SimpleBot{Name: name, RedisClient: redisClient}
}

// HandleMessage processes incoming messages and returns a response
func (b *SimpleBot) HandleMessage(c telebot.Context) string {
	if c.Text() == "/look" {
		botutils.HandleLook(c)
		return ""
	}
	if c.Text() == "/ps" {
		botutils.HandlePS(b.RedisClient, c)
		return ""
	}
	return ""
}
