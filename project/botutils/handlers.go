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
		priceOfchain, _, _:=GetFirstOnSalePrice(redisClient)
		priceOnchain, _, _ :=GetMinPriceFloor(redisClient) 
		price := Min(priceOfchain, priceOnchain)
		priceg, _, _ := GetMinPriceGreen(redisClient)
		startprofit := (price/1000 - 1.4) / 1.4 * 100
		endprofit := (price/1000 - priceg) / priceg * 100

		sendProgress := func(text string, flag bool) {
			txt := 
				fmt.Sprintf("Флор на Heart Locket: %.2f\n", price)+
							"----------------\n"+
				fmt.Sprintf("минт: 1.4\nпрофит: %.2f%%\n", startprofit)+
				"----------------\n"+
				fmt.Sprintf("флор кусочков: %.2f\nпрофит: %.2f%%\n", priceg, endprofit)+
				"----------------\n"+
				fmt.Sprintf("Средняя цена всех NFT: ...\nпрофит сообщества: %s\n", text)+
				"----------------\n"
			if flag {
				txt = text
			}
			if c == nil || c.Message() == nil || c.Chat() == nil {
				return
			}
			chat := c.Chat()
			if sentMsgID == nil {
				msg, err := c.Bot().Send(
					chat,
					txt,
					&telebot.SendOptions{ThreadID: c.Message().ThreadID},
				)
				if err != nil {
					log.Printf("Ошибка отправки: %v", err)
				return
				}
				sentMsgID = msg
			} else {
				c.Bot().Edit(sentMsgID, txt)
			}
		}

		avgPrice, _ := GetAveragePrice(redisClient, sendProgress)
		avgProfit := (price/1000 - avgPrice) / avgPrice * 100
		
		if c != nil {
			fmt.Printf("Ответил в чат %d\n", c.Chat().ID)
		}
		msg := 
				fmt.Sprintf("Флор на Heart Locket: %.2f\n", price)+
							"----------------\n"+
				fmt.Sprintf("минт: 1.4\nпрофит: %.2f%%\n", startprofit)+
				"----------------\n"+
				fmt.Sprintf("флор кусочков: %.2f\nпрофит: %.2f%%\n", priceg, endprofit)+
				"----------------\n"+
				fmt.Sprintf("Средняя цена всех NFT: %.2f\nпрофит сообщества: %.2f%%\n", avgPrice, avgProfit)+
				"----------------\n"

		sendProgress(msg, false)
		return msg
	}

	
func HandleFloorCheckNoCache(redisClient *redis.Client,c telebot.Context, ) string {
		var sentMsgID *telebot.Message = nil
		priceOfchain, _, _:=GetFirstOnSalePrice(redisClient)
		priceOnchain, _, _:=GetMinPriceFloor(redisClient) 
		price := Min(priceOfchain, priceOnchain)
		priceg, _, _ := GetMinPriceGreen(redisClient)
		startprofit := (price/1000 - 1.4) / 1.4 * 100
		endprofit := (price/1000 - priceg) / priceg * 100
		sendProgress := func(text string, flag bool) {
			txt := 
				fmt.Sprintf("Флор на Heart Locket: %.2f\n", price)+
							"----------------\n"+
				fmt.Sprintf("минт: 1.4\nпрофит: %.2f%%\n", startprofit)+
				"----------------\n"+
				fmt.Sprintf("флор кусочков: %.2f\nпрофит: %.2f%%\n", priceg, endprofit)+
				"----------------\n"+
				fmt.Sprintf("Средняя цена всех NFT: ...\nпрофит сообщества: %s\n", text)+
				"----------------\n"
			
			if flag {
				txt = text
			}
			if c == nil || c.Message() == nil || c.Chat() == nil {
				return
			}
			chat := c.Chat()
			if sentMsgID == nil {
				msg, err := c.Bot().Send(
					chat,
					txt,
					&telebot.SendOptions{ThreadID: c.Message().ThreadID},
				)
				if err != nil {
					log.Printf("Ошибка отправки: %v", err)
				return
				}
				sentMsgID = msg
			} else {
				c.Bot().Edit(sentMsgID, txt)
			}
		}

		avgPrice, _ := GetAveragePriceNoCache(redisClient, sendProgress)
		avgProfit := (price/1000 - avgPrice) / avgPrice * 100

		msg := 
				fmt.Sprintf("Флор на Heart Locket: %.2f\n", price)+
							"----------------\n"+
				fmt.Sprintf("минт: 1.4\nпрофит: %.2f%%\n", startprofit)+
				"----------------\n"+
				fmt.Sprintf("флор кусочков: %.2f\nпрофит: %.2f%%\n", priceg, endprofit)+
				"----------------\n"+
				fmt.Sprintf("Средняя цена всех NFT: %.2f\nпрофит сообщества: %.2f%%\n", avgPrice, avgProfit)+
				"----------------\n"

		sendProgress(msg, true)
		return msg
	}

func Min(priceOfchain, priceOnchain float64) float64 {
	fmt.Print("Min ", priceOfchain, priceOnchain)
	if priceOfchain < priceOnchain {
		return priceOfchain
	}
	return priceOnchain
}