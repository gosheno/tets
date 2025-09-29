package getgems

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	apiKey          = "1759093403351-mainnet-9508047-r-Rodf2lVdSIS22ecj6daQcNQTSZLiGTJmmrfJi0Xo2gdppFPU"
	baseURL         = "https://api.getgems.io/public-api/v1/nft/history/"
	defaultPrice    = 1.4
	requestInterval = 760 * time.Millisecond
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

// Получение последней цены по адресу NFT
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

	switch lastItem.TypeData.Type {
	case "sold":
		price, err := strconv.ParseFloat(lastItem.TypeData.Price, 64)
		if err != nil || price <= 0 {
			log.Printf("⚠️ Некорректная цена для NFT %s, используем defaultPrice %.2f\n", address, defaultPrice)
			return defaultPrice
		}
		return price
	case "mint":
		return defaultPrice
	default:
		log.Printf("⚠️ Неизвестный тип транзакции '%s' для NFT %s, используем defaultPrice %.2f\n",
			lastItem.TypeData.Type, address, defaultPrice)
		return defaultPrice
	}
}

// GetAveragePrice читает адреса из файла и возвращает среднюю цену всех NFT
func GetAveragePrice() float64 {
	file, err := os.Open("nft_addresses.txt")
	if err != nil {
		log.Println("❌ Ошибка открытия файла:", err)
		return defaultPrice
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
		return defaultPrice
	}

	// Сброс сканера для повторного чтения
	file.Seek(0, 0)
	scanner = bufio.NewScanner(file)

	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	var sum float64
	var count int

	for scanner.Scan() {
		address := scanner.Text()
		<-ticker.C // Ждем перед запросом
		lastPrice := getLastPrice(address)
		sum += lastPrice
		count++
		if count%100 == 0 || count == total {
			log.Printf("📊 Обработано %d из %d NFT, текущая средняя цена: %.2f TON",
				count, total, sum/float64(count))
		}
	}

	if err := scanner.Err(); err != nil {
		log.Println("❌ Ошибка чтения файла:", err)
		return defaultPrice
	}

	if count == 0 {
		log.Println("❌ Нет NFT для расчета средней цены")
		return defaultPrice
	}

	avgPrice := sum / float64(count)
	return avgPrice
}
