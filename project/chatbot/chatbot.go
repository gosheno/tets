package chatbot

import (
	"context"
	"strings"
	"tg-getgems-bot/botutils"

	"github.com/go-redis/redis/v8"
	"gopkg.in/telebot.v3"
)

// --- Контекст для Redis ---
var Ctx = context.Background()

// --- Тип обработчика команды ---
type BotHandler func(c telebot.Context)

// --- Хранение команды с описанием ---
type CommandInfo struct {
	Handler     BotHandler
	Description string
}

// --- Базовый бот ---
type SimpleBot struct {
	Name        string
	RedisClient *redis.Client
}

// --- Создание SimpleBot ---
func NewSimpleBot(name string, redisClient *redis.Client) *SimpleBot {
	return &SimpleBot{Name: name, RedisClient: redisClient}
}

// --- Реестр команд ---
var commandRegistry = make(map[string]CommandInfo)

// --- Регистрация команды ---
func RegisterCommand(cmd string, handler BotHandler, description string) {
	commandRegistry[cmd] = CommandInfo{
		Handler:     handler,
		Description: description,
	}
}

// --- Превращаем func(c telebot.Context) error в BotHandler ---
func WrapHandlerWithError(f func(c telebot.Context) error) BotHandler {
	return func(c telebot.Context) {
		_ = f(c)
	}
}

// --- Получение списка команд для /help ---
func GetRegisteredCommands() []string {
	cmds := make([]string, 0, len(commandRegistry))
	for cmd, info := range commandRegistry {
		cmds = append(cmds, cmd+" — "+info.Description)
	}
	return cmds
}

func OnTextGlobalHandler(bot *telebot.Bot, redisClient *redis.Client, cb *SimpleBot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		text := c.Text()

		// --- /help ---
		if text == "/help" {
			cmds := GetRegisteredCommands()
			c.Reply("Доступные команды:\n" + strings.Join(cmds, "\n"))
			return nil
		}

		// --- Обработка команд из реестра ---
		for cmd, info := range commandRegistry {
			if strings.HasPrefix(text, cmd) {
				info.Handler(c)
				return nil
			}
		}

		return nil
	}
}


// --- Инициализация команд ---
func InitCommands(bot *SimpleBot) {
	rc := bot.RedisClient

	RegisterCommand("/look", WrapHandlerWithError(func(c telebot.Context) error {
		return botutils.HandleLook(c)
	}),"")

	RegisterCommand("/floor", WrapHandlerWithError(func(c telebot.Context) error {
		return botutils.HandleFloor(c.Bot(), rc, c)
	}), "сводка")

	RegisterCommand("/ps", WrapHandlerWithError(func(c telebot.Context) error {
		return botutils.HandlePS(rc, c)
	}), "")

	RegisterCommand("/address", WrapHandlerWithError(botutils.HandleMeSingleLine(rc)), "Профиль")
}
