package chatbot

import (
	"context"
	"fmt"
	"log"
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


// --- Состояния пользователей для /me ---
var waitingForAddress = make(map[int64]bool)

func WaitingForAddress(userID int64) bool {
	return waitingForAddress[userID]
}
// --- Обработчик /me ---
func HandleMe(redisClient *redis.Client) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID
		waitingForAddress[userID] = true

		msg, err := c.Bot().Reply(c.Message(), "🔑 Пришлите TON-адрес кошелька")
		if err != nil {
			return err
		}

		meMessageID[userID] = msg.ID

		return nil
	}
}

// --- Глобальный текстовый обработчик ---
// Нужно подключить один раз при старте бота:
// bot.Handle(telebot.OnText, OnTextHandler(bot.RedisClient))

// --- Состояние ожидания адреса для /me ---

// --- ID сообщения "пришлите адрес" для каждого пользователя ---
var meMessageID = make(map[int64]int)
func OnTextGlobalHandler(bot *telebot.Bot, redisClient *redis.Client, cb *SimpleBot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID
		text := c.Text()

		// --- Если пользователь в состоянии ожидания адреса для /me
		if waitingForAddress[userID] {
			ownerAddress := strings.TrimSpace(text)
			if len(ownerAddress) < 20 {
				c.Reply("❌ Неверный адрес")
				return nil
			}

			avgPrice, count, err := botutils.GetOwnerAvgBuyPrice(redisClient, ownerAddress)
			if err != nil {
				log.Println("❌ /me error:", err)
				c.Reply("Ошибка при получении данных")
				delete(waitingForAddress, userID)
				return nil
			}

			if count == 0 {
				c.Reply("У вас нет NFT из этой коллекции")
				delete(waitingForAddress, userID)
				return nil
			}

			currentPrice, err := botutils.GetMinPrice(redisClient)
			if err != nil {
				c.Reply("Не удалось получить текущую цену флора")
				delete(waitingForAddress, userID)
				return nil
			}

			pnl := (currentPrice/1000 - avgPrice) / avgPrice * 100
			text := fmt.Sprintf(
				"👤 Ваш профиль\n\nNFT: %d\nСредняя цена покупки: %.2f TON\nHeart Locket: %.2f TON\nВаш PNL: %.2f%%",
				count, avgPrice, currentPrice, pnl,
			)

			// удаляем сообщение "пришлите адрес"
			if msgID, ok := meMessageID[userID]; ok {
				bot.Delete(&telebot.Message{ID: msgID, Chat: c.Chat()})
				delete(meMessageID, userID)
			}

			c.Reply(text)
			delete(waitingForAddress, userID)
			return nil
		}

		// --- Обработка команд ---
		if info, ok := commandRegistry[text]; ok {
			info.Handler(c)
			return nil
		}

		// --- /help ---
		if text == "/help" {
			cmds := GetRegisteredCommands()
			c.Reply("Доступные команды:\n" + strings.Join(cmds, "\n"))
			return nil
		}

		// --- Неизвестная команда ---
		c.Reply("Неизвестная команда. Попробуйте /help")
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

	RegisterCommand("/me", WrapHandlerWithError(HandleMe(rc)), "Профиль")
}
