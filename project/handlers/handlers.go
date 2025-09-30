package handlers

import (
	"fmt"
	"tg-getgems-bot/getgems"

	"github.com/go-redis/redis/v8"
	"gopkg.in/telebot.v3"
)

// HandleFloorCheck processes /floor and /check commands for SimpleBot
func HandleFloorCheck(
	user string,
	redisClient *redis.Client,
	send func(text string) int,
	redact func(msgID int, text string),
	c telebot.Context,
) string {
	price, _, _ := getgems.GetMinPrice(redisClient)
	priceg, _, _ := getgems.GetMinPriceGreen(redisClient)
	startprofit := (price/1000 - 1.4) / 1.4 * 100
	endprofit := (price/1000 - priceg) / priceg * 100
	var progressMsg string = "ща чекну"
	var sentMsgID int
	sendProgress := func(text string) {
		if sentMsgID == 0 {
			sentMsgID = send(text)
		} else {
			redact(sentMsgID, text)
		}
	}
	avgPrice, isCached := getgems.GetAveragePrice(redisClient, sendProgress)
	avgProfit := (price/1000 - avgPrice) / avgPrice * 100
	if !isCached {
		return "Средняя цена всех NFT: [⏳ Обработка...] (может занять до минуты, повторите команду позже)\n" + progressMsg
	}
	return fmt.Sprintf(
		"цена минта: 1.4\nфлор на Heart Locket Reactor: %.4f\nфлор на кусочек: %.4f\nСредняя цена всех NFT: %.2f TON\n----------------\nпрофит по цене минта: %.2f%%\nпрофит по флору кусочков: %.2f%%\nсредний профит комьюнити: %.2f%%",
		price, priceg, avgPrice, startprofit, endprofit, avgProfit)
}
