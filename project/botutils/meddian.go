// GetAveragePriceNoCache читает адреса из файла и возвращает среднюю цену всех NFT без кеширования
package botutils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	apiqueue "tg-getgems-bot/api"
	"time"

	"github.com/go-redis/redis/v8"
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
func GetAveragePrice(
	rdb *redis.Client,
	collectionAddress string,
) (float64, bool) {

	sumKey := "collection:sum:" + collectionAddress
	countKey := "collection:count:" + collectionAddress

	sum, err := rdb.Get(Ctx, sumKey).Float64()
	if err != nil {
		log.Println("❌ redis sum error:", err)
		return defaultPrice, false
	}

	count, err := rdb.Get(Ctx, countKey).Int64()
	if err != nil || count == 0 {
		log.Println("❌ redis count error:", err)
		return defaultPrice, false
	}

	avg := sum / float64(count)
	return avg, true
}

type CollectionHistoryResponse struct {
	Success  bool `json:"success"`
	Response struct {
		Cursor string                `json:"cursor"`
		Items  []CollectionHistoryItem `json:"items"`
	} `json:"response"`
}

type CollectionHistoryItem struct {
	Address           string `json:"address"`
	Name              string `json:"name"`
	Time              string `json:"time"`
	Timestamp         int64  `json:"timestamp"`
	CollectionAddress string `json:"collectionAddress"`
	Lt                string `json:"lt"`
	Hash              string `json:"hash"`
	IsOffchain        bool   `json:"isOffchain"`
	TypeData          TypeData `json:"typeData"`
}

type TypeData struct {
	Type                 string `json:"type"`
	Price                string `json:"price"`      // "1.4"
	PriceNano            string `json:"priceNano"`  // "1400000000"
	NewOwner             string `json:"newOwner"`
	OldOwner             string `json:"oldOwner"`
	RejectFromGlobalTop  bool   `json:"rejectFromGlobalTop"`
	Currency             string `json:"currency"`   // "TON"
}
func dayKey(ts int64) string {
	t := time.UnixMilli(ts).UTC()
	return t.Format("20060102")
}

func monthKey(ts int64) string {
	t := time.UnixMilli(ts).UTC()
	return t.Format("200601")
}

func weekKey(ts int64) string {
	t := time.UnixMilli(ts).UTC()
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d%02d", year, week)
}
func extractPrice(item CollectionHistoryItem) (float64, bool) {
	if item.TypeData.Type != "sold" {
		return 0, false
	}

	// валюта должна быть TON
	if item.TypeData.Currency != "TON" {
		return 0, false
	}

	// приоритет — priceNano
	if item.TypeData.PriceNano != "" {
		nano, err := strconv.ParseInt(item.TypeData.PriceNano, 10, 64)
		if err != nil {
			return 0, false
		}
		return float64(nano) / 1e9, true
	}

	// fallback — price
	if item.TypeData.Price != "" {
		price, err := strconv.ParseFloat(item.TypeData.Price, 64)
		if err != nil {
			return 0, false
		}
		return price, true
	}

	return 0, false
}

func GetCollectionHistory(
	collectionAddress string,
	minTime int64,
	cursor string,
) (*CollectionHistoryResponse, error) {

	baseURL := "https://api.getgems.io/public-api/v1/collection/history/" + collectionAddress

	q := url.Values{}
	q.Set("limit", "100")
	q.Set("reverse", "true")
	if minTime > 0 {
		q.Set("minTime", strconv.FormatInt(minTime, 10))
	}
	if cursor != "" {
		q.Set("after", cursor)
	}
	q.Add("types", "mint")
	q.Add("types", "sold")

	// ⚡ фильтруем только нужные события
	reqURL := baseURL + "?" + q.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("accept", "application/json")
	apiKey := os.Getenv("GETGEMS_TOKEN")
	req.Header.Add("Authorization", apiKey)

	// используем очередь
	resp, err := apiqueue.Queue.Enqueue(req, apiqueue.Low)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("collection history error %d: %s", resp.StatusCode, string(body))
	}

	var result CollectionHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("collection history returned success=false")
	}

	return &result, nil
}

func UpdateCollectionIndex(
	rds *redis.Client,
	collectionAddress string,
) error {

	processKey := "process:collection_indexing"
	SetValue(rds, processKey, "running")
	defer func() {
		SetValue(rds, processKey, "idle")
	}()

	ctx := Ctx

	// --- загрузка последнего timestamp ---
	lastTS, err := rds.Get(ctx, "collection:last_ts:"+collectionAddress).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			lastTS = 1
			log.Printf("[Indexer] Нет lastTS в Redis, начнем с 0")
		} else {
			return err
		}
	} else {
		log.Printf("[Indexer] Загружен lastTS из Redis: %d", lastTS)
	}

	cursor := ""
	page := 1
	prevcursor := ""
	maxTS := lastTS
	for {
		if cursor == prevcursor && prevcursor != "" {
			log.Printf("[Indexer] Cursor не меняется, выходим из цикла")
			break
		}
		prevcursor = cursor
		log.Printf("[Indexer] Загружаем страницу %d, cursor=%s, minTime=%d", page, cursor, lastTS)
		resp, err := GetCollectionHistory(collectionAddress, lastTS+1, cursor)
		if err != nil {
			log.Printf("❌ [Indexer] Ошибка GetCollectionHistory: %v", err)
			return err
		}
		if len(resp.Response.Items) == 0 {
			log.Printf("[Indexer] Пустая страница, выходим")
			break
		}

		for _, item := range resp.Response.Items {
			addr := item.Address

			switch item.TypeData.Type {
			case "mint":
				log.Printf("[Indexer][mint] NFT %s — %s, timestamp=%d", addr, item.Name, item.Timestamp)
				// можно сохранять список NFT, если нужно
				// rds.SAdd(ctx, "collection:fragments:"+collectionAddress, addr)

			case "sold":
				price, ok := extractPrice(item)
				if !ok {
					log.Printf("[Indexer][sold] NFT %s — %s, не удалось извлечь цену", addr, item.Name)
					continue
				}

				priceKey := fmt.Sprintf("nft:last_price:%s:%s", collectionAddress, addr)

				oldPrice, err := rds.Get(ctx, priceKey).Float64()
				if err != nil {
					if errors.Is(err, redis.Nil) {
						oldPrice = 0
					} else {
						return err
					}
				}

				if err := rds.Set(ctx, priceKey, price, 0).Err(); err != nil {
					return err
				}

				sumKey := "collection:sum:" + collectionAddress
				countKey := "collection:count:" + collectionAddress

				pipe := rds.TxPipeline()
				if oldPrice == 0 {
					pipe.Incr(ctx, countKey)
					pipe.IncrByFloat(ctx, sumKey, price)
				} else {
					pipe.IncrByFloat(ctx, sumKey, price-oldPrice)
				}
				if _, err := pipe.Exec(ctx); err != nil {
					return err
				}

				day := dayKey(item.Timestamp)
				week := weekKey(item.Timestamp)
				month := monthKey(item.Timestamp)
				pipe2 := rds.TxPipeline()
				pipe2.Incr(ctx, "collection:sales:day:"+day)
				pipe2.Incr(ctx, "collection:sales:week:"+week)
				pipe2.Incr(ctx, "collection:sales:month:"+month)
				if _, err := pipe2.Exec(ctx); err != nil {
					return err
				}

				log.Printf("[Indexer][sold] NFT %s — %s, oldPrice=%.4f, newPrice=%.4f", addr, item.Name, oldPrice, price)

				saleData, _ := json.Marshal(struct {
					Address   string  `json:"address"`
					Name      string  `json:"name"`
					Price     float64 `json:"price"`
					Timestamp int64   `json:"timestamp"`
				}{
					Address:   addr,
					Name:      item.Name,
					Price:     price,
					Timestamp: item.Timestamp,
				})

				// сохраняем, только если первичная индексация завершена
				indexed, _ := rds.Get(ctx, "collection:"+collectionAddress+":indexed").Result()
				if indexed == "true" {
					rds.RPush(ctx, "collection:new_sales", saleData)
				}
			
			}

			// обновляем timestamp
			if item.Timestamp > maxTS {
				maxTS = item.Timestamp
			}
		}
		lastTS = maxTS
		if resp.Response.Cursor == "" {
			log.Printf("[Indexer] Достигнут конец истории, последняя страница %d", page)
			break
		}
		cursor = resp.Response.Cursor
		page++
		
	}

	// сохраняем последний timestamp
	if err := rds.Set(ctx, "collection:last_ts:"+collectionAddress, lastTS, 0).Err(); err != nil {
		return err
	}
	// ✅ Помечаем, что первичная индексация завершена
	rds.Set(ctx, "collection:"+collectionAddress+":indexed", "true", 0)

	log.Printf("[Indexer] Индексация завершена, сохранен lastTS=%d", lastTS)

	return nil
}
