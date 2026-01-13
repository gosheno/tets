package main

import (
	"fmt"
	"log"
	"os"
	apiqueue "tg-getgems-bot/api"
	"tg-getgems-bot/botutils"
	"tg-getgems-bot/chatbot"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/telebot.v3"
)

// ------------------ структуры под JSON ------------------

type ApiResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Attributes []Attribute `json:"attributes"`
	} `json:"response"`
}
type ApiResponseGreen struct {
	Success  bool      `json:"success"`
	Response AttrGreen `json:"response"`
}

type AttrGreen struct {
	FloorPrice          float64 `json:"floorPrice"`
	FloorPriceNano      string  `json:"floorPriceNano"` // в JSON это строка
	ItemsCount          int     `json:"itemsCount"`
	TotalVolumeSold     string  `json:"totalVolumeSold"`
	TotalVolumeSoldNano string  `json:"totalVolumeSoldNano"`
	Holders             int     `json:"holders"`
}

type Attribute struct {
	TraitType string       `json:"traitType"`
	Values    []AttrValues `json:"values"`
}

type AttrValues struct {
	Value        string `json:"value"`
	Count        int    `json:"count"`
	MinPrice     string `json:"minPrice"`
	MinPriceNano string `json:"minPriceNano"`
}

// ------------------ запуск бота ------------------

func main() {
	// Загружаем .env
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env файл не найден, используем переменные окружения")
	}

	pref := telebot.Settings{
		Token:  os.Getenv("TELEGRAM_TOKEN"),
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}
	bot, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatal(err)
	}

	// Инициализация Redis-клиента
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := 0
	cb := chatbot.NewSimpleBot("MyBot", botutils.NewRedisClient(redisAddr, redisPassword, redisDB))

	// Обработка всех текстовых сообщений через chatbot
	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		cb.HandleMessage(c)
		return nil
	})
	cb.RedisClient.FlushAll(botutils.Ctx)
	apiqueue.InitPriorityQueue(100, 100, 1200*time.Millisecond)

	// Запуск /floor раз в час
	go func() {
		var msg *telebot.Message
		for {
			adminID := os.Getenv("CHAT_ID")
			threadid := os.Getenv("THREAD_ID")
			
			if adminID == "" {
				log.Println("ADMIN_CHAT_ID не задан, /floor не будет отправлен")
			} else {
				id := parseChatID(adminID)
				thr := parseTreadID(threadid)
				chat := &telebot.Chat{ID: id}
				_, imgPath := botutils.HandleFloorCheck(cb.RedisClient, nil)
				if msg != nil {
					bot.Delete(msg)
				}
				photo := &telebot.Photo{File: telebot.FromDisk(imgPath)}
				msg,  err = bot.Send(chat, photo, &telebot.SendOptions{ThreadID: thr})
				if err != nil {
					log.Printf("Ошибка отправки /floor: %v", err)
				}
			}
			time.Sleep(3 * time.Hour)
		}
	}()
	log.Println("Бот запущен")
	bot.Start()
}

func parseChatID(s string) int64 {
	var id int64
	fmt.Sscan(s, &id)
	return id
}
func parseTreadID(s string) int {
	var id int
	fmt.Sscan(s, &id)
	return id
}
