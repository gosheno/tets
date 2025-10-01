package botutils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	apiqueue "tg-getgems-bot/api"

	"github.com/go-redis/redis/v8"
)

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
	FloorPriceNano      string  `json:"floorPriceNano"`
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

func GetMinPrice(redisClient *redis.Client) (float64, []byte, error) {
	cacheKey := "min_price_reactor"
	cached, err := GetValue(redisClient, cacheKey)
	if err == nil && cached != "" {
		price, err := strconv.ParseFloat(cached, 64)
		if err == nil {
			fmt.Println("[Redis] Возврат из кеша min_price_reactor:", price)
			return price, []byte(fmt.Sprintf("{\"cached\":true,\"price\":%f}", price)), nil
		}
	}
	

	url := "https://api.getgems.io/public-api/v1/collection/attributes/EQC4XEulxb05Le5gF6esMtDWT5XZ6tlzlMBQGNsqffxpdC5U"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("Authorization", os.Getenv("GETGEMS_TOKEN"))
	resp, err := apiqueue.Queue.Enqueue(req, apiqueue.High)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("status %s", resp.Status)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	var data ApiResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return 0, bodyBytes, err
	}
	for _, attr := range data.Response.Attributes {
		for _, v := range attr.Values {
			if v.Value == "Reactor" {
				price, err := strconv.ParseFloat(v.MinPrice, 64)
				if err != nil {
					return 0, bodyBytes, err
				}
				// Кэшируем значение на 1 час
				fmt.Print("[API] min_price_reactor: ", price, "\n")
				redisClient.Set(Ctx, cacheKey, v.MinPrice, 3600*1_000_000_000) // 1 час в наносекундах
				return price, bodyBytes, nil
			}
		}
	}
	print(bodyBytes)
	return 0, bodyBytes, fmt.Errorf("не найден model.reactor.MinPriceNano")
}

func GetMinPriceGreen(redisClient *redis.Client) (float64, []byte, error) {
	cacheKey := "min_price_green"
	cached, err := GetValue(redisClient, cacheKey)
	if err == nil && cached != "" {
		price, err := strconv.ParseFloat(cached, 64)
		if err == nil {
			fmt.Println("[Redis] Возврат из кеша min_price_green:", price)
			return price, []byte(fmt.Sprintf("{\"cached\":true,\"price\":%f}", price)), nil
		}
	}
	fmt.Println("[API] Запрос min_price_green")

	url := "https://api.getgems.io/public-api/v1/collection/stats/EQAnmo8tBH8gSErzWDrdlJiF8kxgfJEynKMIBxL2MkuHvPBc"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("Authorization", os.Getenv("GETGEMS_TOKEN"))
	resp, err := apiqueue.Queue.Enqueue(req, apiqueue.High)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("status %s", resp.Status)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	var data ApiResponseGreen
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return 0, bodyBytes, err
	}

	// Кэшируем значение на 5 часов
	fmt.Print("[API] min_price: ", data.Response.FloorPrice, "\n")
	redisClient.Set(Ctx, cacheKey, fmt.Sprintf("%f", data.Response.FloorPrice), 18000*1_000_000_000) // 1 час в наносекундах
	return data.Response.FloorPrice, bodyBytes, nil
}

