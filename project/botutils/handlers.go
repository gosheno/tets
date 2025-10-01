package botutils

import (
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
	"gopkg.in/telebot.v3"
)

// HandleFloorCheck processes /floor and /check commands for SimpleBot
func HandleFloorCheck(redisClient *redis.Client,c telebot.Context,) string {
		var sentMsgID *telebot.Message = nil
		
		sendProgress := func(text string) {
			if c == nil || c.Message() == nil || c.Chat() == nil {
				return
			}
			chat := c.Chat()
			if sentMsgID == nil {
				msg, err := c.Bot().Send(
					chat,
					text,
					&telebot.SendOptions{ThreadID: c.Message().ThreadID},
				)
				if err != nil {
					log.Printf("Ошибка отправки: %v", err)
				return
				}
				sentMsgID = msg
			} else {
				c.Bot().Edit(sentMsgID, text)
			}
		}
		
		avgPrice, _ := GetAveragePrice(redisClient, sendProgress)
		price, _, _ := GetMinPrice(redisClient)
		priceg, _, _ := GetMinPriceGreen(redisClient)
		startprofit := (price/1000 - 1.4) / 1.4 * 100
		endprofit := (price/1000 - priceg) / priceg * 100
		avgProfit := (price/1000 - avgPrice) / avgPrice * 100
		
		if c != nil {
			fmt.Printf("Ответил в чат %d\n", c.Chat().ID)
		}
		msg := fmt.Sprintf(
			"цена минта: 1.4\nфлор на Heart Locket Reactor: %.2f\nфлор на кусочек: %.2f\nСредняя цена всех NFT: %.2f TON\n----------------\nпрофит по цене минта: %.2f%%\nпрофит по флору кусочков: %.2f%%\nсредний профит комьюнити: %.2f%%",
			price, priceg, avgPrice, startprofit, endprofit, avgProfit)

		sendProgress(msg)
		return msg
	}

	
func HandleFloorCheckNoCache(redisClient *redis.Client,c telebot.Context,) string {
		var sentMsgID *telebot.Message = nil
		sendProgress := func(text string) {
			if sentMsgID == nil {
				
				chat := c.Chat()
				msg, err := c.Bot().Send(
					chat,
					text,
					&telebot.SendOptions{ThreadID: c.Message().ThreadID,},
				)
				if err != nil {
					log.Printf("Ошибка отправки: %v", err)
					return
				}
				sentMsgID = msg

			} else {
				c.Bot().Edit(sentMsgID, text)
			}
		}

		avgPrice, _ := GetAveragePriceNoCache(redisClient, sendProgress)
		price, _, _ := GetMinPrice(redisClient)
		priceg, _, _ := GetMinPriceGreen(redisClient)
		startprofit := (price/1000 - 1.4) / 1.4 * 100
		endprofit := (price/1000 - priceg) / priceg * 100
		avgProfit := (price/1000 - avgPrice) / avgPrice * 100

		msg := fmt.Sprintf(
			"цена минта: 1.4\nфлор на Heart Locket Reactor: %.2f\nфлор на кусочек: %.2f\nСредняя цена всех NFT: %.2f TON\n----------------\nпрофит по цене минта: %.2f%%\nпрофит по флору кусочков: %.2f%%\nсредний профит комьюнити: %.2f%%",
			price, priceg, avgPrice, startprofit, endprofit, avgProfit)

		sendProgress(msg)
		return msg
	}