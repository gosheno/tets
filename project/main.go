package main

import (
	"fmt"
	"log"
	"os"
	apiqueue "tg-getgems-bot/api"
	"tg-getgems-bot/botutils"
	"tg-getgems-bot/chatbot"
	"time"

	"github.com/go-redis/redis/v8"
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
func startCollectionIndexer(rdb *redis.Client, collection string) {
	const updateInterval = 1 * time.Minute
	ctx := botutils.Ctx
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	firstRun := true

	for {
		// Redis lock, чтобы один индексатор на коллекцию
		lockKey := "lock:collection_index:" + collection
		ok, err := rdb.SetNX(ctx, lockKey, 1, 5*time.Minute).Result()
		if err != nil {
			log.Println("❌ Redis lock error:", err)
			<-ticker.C
			continue
		}
		if !ok {
			// Кто-то другой уже индексирует
			<-ticker.C
			continue
		}

		// Логируем первый прогон
		if firstRun {
			log.Printf("🚀 Первичный прогон индексации коллекции %s...", collection)
			firstRun = false
			rdb.Set(ctx, "collection:"+collection+":indexed", "false", 0)
		}

		// Запускаем UpdateCollectionIndex
		err = botutils.UpdateCollectionIndex(rdb, collection)
		if err != nil {
			log.Println("❌ indexer error:", err)
		}

		// Освобождаем lock
		rdb.Del(ctx, lockKey)

		// Ждём интервал
		<-ticker.C
	}
}


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

	// Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := 0
	redisClient := botutils.NewRedisClient(redisAddr, redisPassword, redisDB)

	cb := chatbot.NewSimpleBot("MyBot", redisClient)

	// --- Инициализация команд ---
	chatbot.InitCommands(cb)

	// --- Глобальный текстовый обработчик ---
	bot.Handle(telebot.OnText, chatbot.OnTextGlobalHandler(bot, cb.RedisClient, cb))

	// Очистка Redis для теста (необязательно в продакшене)
	cb.RedisClient.FlushAll(botutils.Ctx)

	// Инициализация очереди API
	apiqueue.InitPriorityQueue(100, 100, 1200*time.Millisecond)

	collection := os.Getenv("COLLECTION_ADDRESS")
	if collection == "" {
		log.Println("⚠️ COLLECTION_ADDRESS не задан — индексатор не запущен")
	} else {
		go startCollectionIndexer(cb.RedisClient, collection)
	}

	go botutils.NotifyNewSales(bot, cb.RedisClient, collection)

	// Запуск /floor раз в 3 часа
	go func() {
		var msg *telebot.Message
		for {
			indexed, _ := cb.RedisClient.Get(botutils.Ctx, "collection:"+collection+":indexed").Result()
			if indexed != "true" {
				log.Println("[Floor] Первичная индексация ещё не завершена, ждём 30 секунд...")
				time.Sleep(30 * time.Second)
				continue
			}

			textMsg, imgPath := botutils.FloorCheck(cb.RedisClient)

			if msg != nil {
				bot.Delete(msg)
			}

			adminID := os.Getenv("CHAT_ID")
			threadID := os.Getenv("THREAD_ID")
			chat := &telebot.Chat{ID: parseChatID(adminID)}
			thread := parseThreadID(threadID)

			if imgPath != "" {
				photo := &telebot.Photo{File: telebot.FromDisk(imgPath)}
				msg, err = bot.Send(chat, photo, &telebot.SendOptions{ThreadID: thread})
				if err != nil {
					log.Printf("Ошибка отправки /floor (картинка): %v", err)
					bot.Send(chat, textMsg, &telebot.SendOptions{ThreadID: thread})
				}
			} else {
				msg, err = bot.Send(chat, textMsg, &telebot.SendOptions{ThreadID: thread})
				if err != nil {
					log.Printf("Ошибка отправки /floor (текст): %v", err)
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

func parseThreadID(s string) int {
	var id int
	fmt.Sscan(s, &id)
	return id
}
func parseTreadID(s string) int {
	var id int
	fmt.Sscan(s, &id)
	return id
}
