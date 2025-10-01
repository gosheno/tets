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
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	apiKey          = "1759093403351-mainnet-9508047-r-Rodf2lVdSIS22ecj6daQcNQTSZLiGTJmmrfJi0Xo2gdppFPU"
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
	req.Header.Add("Authorization", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
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
func GetAveragePrice(redisClient *redis.Client, sendProgress func(text string)) (float64, bool) {
	cacheKey := "nft_avg_price"
	cached, err := GetValue(redisClient, cacheKey)
	sendProgress("ща чекну")
	time.Sleep(1 * time.Second) // Небольшая пауза для UX
	if err == nil && cached != "" {
		price, err := strconv.ParseFloat(cached, 64)
		if err == nil {
			fmt.Println("[Redis] Возврат из кеша средней цены:", price)
			return price, true
		}
	}
	return GetAveragePriceNoCache(redisClient, sendProgress)
}

func GetAveragePriceNoCache(redisClient *redis.Client, sendProgress func(text string)) (float64, bool) {
	cacheKey := "nft_avg_price"
	file, err := os.Open("nft_addresses.txt")
	sendProgress("придется подождать, считываю адреса..." + "\n" + "📊 Обработано 0 из 1000 NFT")

	if err != nil {
		log.Println("❌ Ошибка открытия файла:", err)
		sendProgress("Ошибка открытия файла адресов")
		return defaultPrice, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Подсчёт общего количества строк для прогресса
	var total int
	for scanner.Scan() {
		total++
	}
	if err := scanner.Err(); err != nil {
		log.Println("❌ Ошибка чтения файла:", err)
		sendProgress("Ошибка чтения файла адресов")
		return defaultPrice, false
	}

	// Сброс сканера для повторного чтения
	file.Seek(0, 0)
	scanner = bufio.NewScanner(file)

	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	var sum float64
	var count int
	log.Printf("📊 Обработка NFT начата")
	for scanner.Scan() {
		address := scanner.Text()
		<-ticker.C // Ждем перед запросом
		lastPrice := getLastPrice(address)
		sum += lastPrice
		count++
		if count%10 == 0 || count == total {
			
			var msg = fmt.Sprintf("придется подождать, считываю адреса..." + "\n" + "📊 Обработано %d из %d NFT", count, total)
			sendProgress(msg)
			log.Printf("📊 Обработано %d из %d NFT, текущая средняя цена: %.2f TON",
				count, total, sum/float64(count))
		}
		if count ==20{
			break
		}
	}

	if err := scanner.Err(); err != nil {
		log.Println("❌ Ошибка чтения файла:", err)
		return defaultPrice, false
	}

	log.Printf("📊 Обработка NFT")
	sendProgress("📊 Обработка завершена")
	time.Sleep(1 * time.Second) // Небольшая пауза для UX
	if count == 0 {
		return defaultPrice, false
	}
	avgPrice := sum / float64(count)
	err = redisClient.Set(Ctx, cacheKey, fmt.Sprintf("%f", avgPrice), time.Hour*10).Err()
	if err != nil {
		log.Println("❌ Ошибка установки значения в Redis:", err)
	}
	return avgPrice, false
}