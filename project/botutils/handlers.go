package botutils

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"gopkg.in/telebot.v3"
)

func FloorCheck(redisClient *redis.Client) (string, string) {
    collectionAddress := os.Getenv("COLLECTION_ADDRESS")
    if collectionAddress == "" {
        return "⚠️ COLLECTION_ADDRESS не задан", ""
    }

    // --- Ждем завершения первичной индексации с таймаутом 10 минут ---
    timeout := time.After(10 * time.Minute)
    tick := time.Tick(30 * time.Second)

    for {
        indexed, err := redisClient.Get(Ctx, "collection:"+collectionAddress+":indexed").Result()
        if err != nil && !errors.Is(err, redis.Nil) {
            log.Printf("[Floor] Redis error при проверке индексации: %v", err)
            return "Ошибка Redis", ""
        }
        if indexed == "true" {
            break // индексация завершена
        }

        log.Println("[Floor] Первичная индексация ещё не завершена, ждём 30 секунд...")

        select {
        case <-timeout:
            log.Println("[Floor] Таймаут ожидания первичной индексации")
            return "Индексация не завершена", ""
        case <-tick:
            continue
        }
    }

    // --- Получение актуальных цен ---
    priceOfchain, _ := GetFirstOnSalePrice(redisClient)
    priceOnchain, _:= GetMinPriceFloor(redisClient)
    price := Min(priceOfchain, priceOnchain)

    priceGreen, _ := GetMinPriceGreen(redisClient)
    priceUSD, _ := GetTonPrice(redisClient)

    // Расчёт прибыли
    startProfit := (price/1000 - 1.4) / 1.4 * 100
    startProfitUSD := (price/1000*priceUSD - 1.4*3.125) / (1.4 * 3.125) * 100
    endProfit := (price/1000 - priceGreen) / priceGreen * 100

    // Средняя цена
    avgPrice, _ := GetAveragePrice(redisClient, collectionAddress)
    avgProfit := (price/1000 - avgPrice) / avgPrice * 100

    // Статистика по покупкам
    count, _ := GetCount(redisClient)

    // --- Формируем текстовое сообщение ---
    msg := fmt.Sprintf(
        "Флор на Heart Locket: %.2f\n----------------\nминт: 1.4\nпрофит: %.2f%%\n----------------\nфлор кусочков: %.2f\nпрофит: %.2f%%\n----------------\nСредняя цена всех NFT: %.2f\nпрофит сообщества: %.2f%%\n----------------\n📊 Статистика покупок фрагментов:\nЗа день: %d\nЗа неделю: %d\nЗа месяц: %d\n",
        price, startProfit, priceGreen, endProfit, avgPrice, avgProfit,
        count.Day, count.Week, count.Month,
    )

    // --- Генерация картинки ---
    imgPath := ""
    imgPath, err := GenerateStatImage(price, startProfit, priceGreen, endProfit, avgPrice, avgProfit, count, priceUSD, startProfitUSD)
    if err != nil {
        log.Printf("[Floor] Ошибка генерации изображения: %v", err)
        imgPath = "" // Если не удалось, вернем пустую строку
    }

    return msg, imgPath
}


func HandleFloor(bot *telebot.Bot, redisClient *redis.Client, c telebot.Context) error {
    chat := c.Chat()
    // Проверяем, завершена ли первичная индексация
    collectionAddress := os.Getenv("COLLECTION_ADDRESS")
    if collectionAddress == "" {
        bot.Send(chat, "⚠️ COLLECTION_ADDRESS не задан", &telebot.SendOptions{ReplyTo: c.Message()})
        return nil
    }

    indexed, err := redisClient.Get(Ctx, "collection:"+collectionAddress+":indexed").Result()
    if err != nil && !errors.Is(err, redis.Nil) {
        log.Printf("[Floor] Redis error: %v", err)
        bot.Send(chat, "Ошибка Redis при проверке индексации", &telebot.SendOptions{ReplyTo: c.Message()})
        return nil
    }

    var waitMsg *telebot.Message
    if indexed != "true" {
        // Отправляем сообщение о том, что нужно подождать
        waitMsg, _ = bot.Send(chat, "⌛ Первичная индексация ещё не завершена, подождите...", &telebot.SendOptions{ReplyTo: c.Message()})
    }

    // Запускаем FloorCheck (ожидает завершения индексации)
    msgText, imgPath := FloorCheck(redisClient)

    // Удаляем сообщение о ожидании, если оно было
    if waitMsg != nil {
        bot.Delete(waitMsg)
    }

    // Отправляем результат пользователю
    if imgPath != "" {
        photo := &telebot.Photo{File: telebot.FromDisk(imgPath)}
        _, err := bot.Send(chat, photo, &telebot.SendOptions{ReplyTo: c.Message()})
        if err != nil {
            log.Printf("[Floor] Ошибка отправки картинки: %v", err)
            bot.Send(chat, msgText, &telebot.SendOptions{ReplyTo: c.Message()})
        }
    } else {
        bot.Send(chat, msgText, &telebot.SendOptions{ReplyTo: c.Message()})
    }

    return nil
}


func Min(priceOfchain, priceOnchain float64) float64 {
	fmt.Print("Min ", priceOfchain, priceOnchain)
	if priceOfchain < priceOnchain {
		return priceOfchain
	}
	return priceOnchain
}

// HandleCount processes /count command
func HandleCount(redisClient *redis.Client, c telebot.Context) error {
	count, err := GetCount(redisClient)
	if err != nil {
		log.Printf("Ошибка получения статистики: %v", err)
		return c.Send("❌ Ошибка получения статистики покупок")
	}

	msg := fmt.Sprintf(
		"📊 Статистика покупок фрагментов:\n"+
			"----------------\n"+
			"За день: %d\n"+
			"За неделю: %d\n"+
			"За месяц: %d\n"+
			"----------------\n",
		count.Day,
		count.Week,
		count.Month,
	)

	return c.Send(msg)
}

