package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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
	FloorPrice        float64 `json:"floorPrice"`
	FloorPriceNano    string  `json:"floorPriceNano"` // в JSON это строка
	ItemsCount        int     `json:"itemsCount"`
	TotalVolumeSold   string  `json:"totalVolumeSold"`
	TotalVolumeSoldNano string `json:"totalVolumeSoldNano"`
	Holders           int     `json:"holders"`
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

// ------------------ логика получения цены ------------------

func getMinPrice() (float64, []byte, error) {
	url := "https://api.getgems.io/public-api/v1/collection/attributes/EQC4XEulxb05Le5gF6esMtDWT5XZ6tlzlMBQGNsqffxpdC5U"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("Authorization", os.Getenv("GETGEMS_TOKEN"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("status %s", resp.Status)
	}

	// читаем тело полностью
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	var data ApiResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return 0, bodyBytes, err
	}

	// ищем model.reactor.minPrice
	for _, attr := range data.Response.Attributes {
			for _, v := range attr.Values {
				if v.Value == "Reactor" {
					price, err := strconv.ParseFloat(v.MinPrice, 64)
					if err != nil {
						return 0, bodyBytes, err
					}
					return price, bodyBytes, nil
				}
			}
	}
	return 0, bodyBytes, fmt.Errorf("не найден model.reactor.MinPriceNano")
}

func getMinPriceGreen() (float64, []byte, error) {
	url := "https://api.getgems.io/public-api/v1/collection/stats/EQAnmo8tBH8gSErzWDrdlJiF8kxgfJEynKMIBxL2MkuHvPBc"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("Authorization", os.Getenv("GETGEMS_TOKEN"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("status %s", resp.Status)
	}

	// читаем тело полностью
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	var data ApiResponseGreen
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return 0, bodyBytes, err
	}

	// ищем model.reactor.minPrice
	return data.Response.FloorPrice, bodyBytes, nil
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

	chatID := os.Getenv("CHAT_ID")

	// команда для проверки вручную
	bot.Handle("/check", func(c telebot.Context) error {
		price, _, err := getMinPrice()
		priceg, _, err := getMinPriceGreen()
		startprofit:= (price/1000 - 1.4)/1.4*100  
		endprofit:= (price/1000 - priceg)/priceg*100
		if err != nil {
			c.Send("Ошибка: " + err.Error())
		} else {
			c.Send(fmt.Sprintf(
				"цена минта: 1.4\nфлор на Heart Locket Reactor: %.4f\nфлор на кусочек: %.4f\n----------------\nпрофит по цене минта: %.2f%%\nпрофит по флору кусочков: %.2f%%",
				 price, priceg, startprofit, endprofit))
				
		}

		return nil
	})

	// авто-проверка каждые 10 минут
	go func() {
		for {
			price, _, err := getMinPrice()
			priceg, _, err := getMinPriceGreen()
			startprofit:= (price/1000 - 1.4)/1.4*100  
			endprofit:= (price/1000 - priceg)/priceg*100
			if err != nil {
				log.Println("Ошибка getMinPrice:", err)
			} else {
				msg := fmt.Sprintf("цена минта: 1.4\nфлор на Heart Locket Reactor: %.4f\nфлор на кусочек: %.4f\n----------------\nпрофит по цене минта: %.2f%%\nпрофит по флору кусочков: %.2f%%",
				price, priceg, startprofit, endprofit)
				
				id := parseChatID(chatID)
				bot.Send(&telebot.Chat{ID: id}, msg)
				
			}
			time.Sleep(1 * time.Minute)
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
