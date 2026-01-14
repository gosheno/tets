package botutils

import (
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
	"gopkg.in/telebot.v3"
)

func HandleFloorCheck(redisClient *redis.Client, c telebot.Context) (string, string) {
	// Проверяем статус сбора данных

	priceOfchain, _, _ := GetFirstOnSalePrice(redisClient)
	priceOnchain, _, _ := GetMinPriceFloor(redisClient)
	price := Min(priceOfchain, priceOnchain)
	priceg, _, _ := GetMinPriceGreen(redisClient)
	priseUsd, _, _ := GetTonPrice()
	startprofit := (price/1000 - 1.4) / 1.4 * 100
	startProfitUsd := (price/1000*priseUsd - 1.4*3.125) / (1.4 * 3.125) * 100
	endprofit := (price/1000 - priceg) / priceg * 100
	// Запрашиваем среднюю цену (это может запустить сбор данных)
	avgPrice, _ := GetAveragePrice(redisClient)

	avgProfit := (price/1000 - avgPrice) / avgPrice * 100

	if c != nil {
		fmt.Printf("Ответил в чат %d\n", c.Chat().ID)
	}
	count, _ := GetCount(redisClient)

	// Возвращаем текстовое сообщение для совместимости
	msg :=
		fmt.Sprintf("Флор на Heart Locket: %.2f\n", price) +
			"----------------\n" +
			fmt.Sprintf("минт: 1.4\nпрофит: %.2f%%\n", startprofit) +
			"----------------\n" +
			fmt.Sprintf("флор кусочков: %.2f\nпрофит: %.2f%%\n", priceg, endprofit) +
			"----------------\n" +
			fmt.Sprintf("Средняя цена всех NFT: %.2f\nпрофит сообщества: %.2f%%\n", avgPrice, avgProfit) +
			"----------------\n" +
			fmt.Sprintf("📊 Статистика покупок фрагментов:\n"+
				"За день: %d\n"+
				"За неделю: %d\n"+
				"За месяц: %d\n", count.Day, count.Week, count.Month)

	// Генерируем картинку со статистикой в конце
	imgPath, err := GenerateStatImage(price, startprofit, priceg, endprofit, avgPrice, avgProfit, count, priseUsd, startProfitUsd)
	if err != nil {
		log.Printf("Ошибка генерации изображения: %v", err)
	}

	return msg, imgPath
}

func HandleFloorCheckNoCache(redisClient *redis.Client, c telebot.Context) string {
	priceOfchain, _, _ := GetFirstOnSalePrice(redisClient)
	priceOnchain, _, _ := GetMinPriceFloor(redisClient)
	price := Min(priceOfchain, priceOnchain)
	priceg, _, _ := GetMinPriceGreen(redisClient)
	startprofit := (price/1000 - 1.4) / 1.4 * 100
	endprofit := (price/1000 - priceg) / priceg * 100

	avgPrice, _ := GetAveragePriceNoCache(redisClient)
	avgProfit := (price/1000 - avgPrice) / avgPrice * 100

	msg :=
		fmt.Sprintf("Флор на Heart Locket: %.2f\n", price) +
			"----------------\n" +
			fmt.Sprintf("минт: 1.4\nпрофит: %.2f%%\n", startprofit) +
			"----------------\n" +
			fmt.Sprintf("флор кусочков: %.2f\nпрофит: %.2f%%\n", priceg, endprofit) +
			"----------------\n" +
			fmt.Sprintf("Средняя цена всех NFT: %.2f\nпрофит сообщества: %.2f%%\n", avgPrice, avgProfit) +
			"----------------\n"
	return msg
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