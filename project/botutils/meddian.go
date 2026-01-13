// GetAveragePriceNoCache читает адреса из файла и возвращает среднюю цену всех NFT без кеширования
package botutils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	apiqueue "tg-getgems-bot/api"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
)

const (
	baseURL         = "https://api.getgems.io/public-api/v1/nft/history/"
	defaultPrice    = 1.4
	requestInterval = 770 * time.Millisecond
)

type Response struct {
	Response struct {
		Items []Item `json:"items"`
	} `json:"response"`
	Success bool `json:"success"`
}

type Item struct {
	TypeData struct {
		Type  string `json:"type"`
		Price string `json:"price,omitempty"`
	} `json:"typeData"`
}

// Получение последней цены по адресу NFT с кешированием
func getLastPrice(address string) float64 {
	url := fmt.Sprintf("%s%s?limit=1&types=mint&types=sold", baseURL, address)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("❌ Ошибка создания запроса:", err)
		return defaultPrice
	}
	req.Header.Add("accept", "application/json")
	apiKey := os.Getenv("GETGEMS_TOKEN")
	req.Header.Add("Authorization", apiKey)
	resp, err := apiqueue.Queue.Enqueue(req, apiqueue.Low)
	if err != nil {
		log.Println("❌ Ошибка HTTP-запроса:", err)
		return defaultPrice
	}
	defer resp.Body.Close()

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Println("❌ Ошибка декодирования JSON:", err)
		return defaultPrice
	}

	if len(result.Response.Items) == 0 {
		log.Printf("⚠️ Нет истории для NFT %s, используем defaultPrice %.2f\n", address, defaultPrice)
		return defaultPrice
	}

	lastItem := result.Response.Items[0]
	var price float64
	switch lastItem.TypeData.Type {
	case "sold":
		price, err = strconv.ParseFloat(lastItem.TypeData.Price, 64)
		if err != nil || price <= 0 {
			log.Printf("⚠️ Некорректная цена для NFT %s, используем defaultPrice %.2f\n", address, defaultPrice)
			price = defaultPrice
		}
	case "mint":
		price = defaultPrice
	default:
		log.Printf("⚠️ Неизвестный тип транзакции '%s' для NFT %s, используем defaultPrice %.2f\n",
			lastItem.TypeData.Type, address, defaultPrice)
		price = defaultPrice
	}
	return price
}

// GetAveragePrice читает адреса из файла и возвращает среднюю цену всех NFT с кешированием
func GetAveragePrice(redisClient *redis.Client) (float64, bool) {
	cacheKey := "nft_avg_price"
	cached, err := GetValue(redisClient, cacheKey)
	time.Sleep(1 * time.Second) // Небольшая пауза для UX
	if err == nil && cached != "" {
		price, err := strconv.ParseFloat(cached, 64)
		if err == nil {
			fmt.Println("[Redis] Возврат из кеша средней цены:", price)
			return price, true
		}
	}
	return GetAveragePriceNoCache(redisClient)
}

var requestGroup singleflight.Group

func GetAveragePriceNoCache(redisClient *redis.Client) (float64, bool) {
	cacheKey := "nft_avg_price"
	file, err := os.Open("nft_addresses.txt")
	
	if err != nil {
		log.Println("❌ Ошибка открытия файла:", err)
		return defaultPrice, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Подсчёт общего количества строк
	var total int
	for scanner.Scan() {
		total++
	}
	if err := scanner.Err(); err != nil {
		log.Println("❌ Ошибка чтения файла:", err)
		return defaultPrice, false
	}

	// Сброс сканера
	file.Seek(0, 0)
	scanner = bufio.NewScanner(file)

	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	var sum float64
	var count int

	log.Printf("📊 Обработка NFT начата")

	sem := make(chan struct{}, 1) // максимум 5 одновременных запросов

	for scanner.Scan() {
		address := scanner.Text()

		addrCacheKey := fmt.Sprintf("nft_price:%s", address)
		cachedPrice, err := redisClient.Get(Ctx, addrCacheKey).Result()
		var lastPrice float64
		if err == nil {
			fmt.Print("[cache] ", address, "\n")
			lastPrice, _ = strconv.ParseFloat(cachedPrice, 64)
		} else {
			// Запрос через singleflight + semaphore
			val, _, _ := requestGroup.Do(addrCacheKey, func() (interface{}, error) {
				sem <- struct{}{}        // блокировка
				defer func() { <-sem }() // освобождение
				<-ticker.C               // задержка между запросами
				lastPrice := getLastPrice(address)
				fmt.Print("[api] ", address, "\n")
				redisClient.Set(Ctx, addrCacheKey, fmt.Sprintf("%f", lastPrice), time.Hour)
				return lastPrice, nil
			})
			lastPrice = val.(float64)
		}

		sum += lastPrice
		count++
		if count == 1 {
			break
		}
		// Прогресс каждые 10 или на последней NFT
		if count%10 == 0 || count == total {
			log.Printf("📊 Прогресс: %d/%d, текущая средняя: %.2f TON", count, total, sum/float64(count))
		}

	}

	if err := scanner.Err(); err != nil {
		log.Println("❌ Ошибка чтения файла:", err)
		return defaultPrice, false
	}

	avgPrice := sum / float64(count)
	// Сохраняем среднюю цену в кэш
	err = redisClient.Set(Ctx, cacheKey, fmt.Sprintf("%f", avgPrice), time.Hour*10).Err()
	if err != nil {
		log.Println("❌ Ошибка установки средней цены в Redis:", err)
	}

	time.Sleep(1 * time.Second)
	return avgPrice, true
}
