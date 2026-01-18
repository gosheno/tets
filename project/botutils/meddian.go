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
	cursor string,
) (*CollectionHistoryResponse, error) {

	baseURL := "https://api.getgems.io/public-api/v1/collection/history/" + collectionAddress

	q := url.Values{}
	q.Set("limit", "100")
	q.Set("reverse", "true")
	if  cursor != "" {
		q.Set("after", cursor)
	}

	q.Add("types", "mint")
	q.Add("types", "sold")

	reqURL := baseURL + "?" + q.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", os.Getenv("GETGEMS_TOKEN"))

	resp, err := apiqueue.Queue.Enqueue(req, apiqueue.Low)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"collection history error %d: %s",
			resp.StatusCode, string(body),
		)
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
	defer SetValue(rds, processKey, "idle")
	primaryKey := "collection:" + collectionAddress + ":primary_index_done"

	isFirst := false
	exists, _ := rds.Exists(Ctx, primaryKey).Result()
	if exists == 0 {
		isFirst = true
		log.Println("[Indexer] Первая индексация")
	}else{
		log.Println("[Indexer] Последующая индексация")
	}
	ctx := Ctx

	// --- lastTS ---
	lastTS, err := rds.Get(ctx, "collection:last_ts:"+collectionAddress).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			lastTS = 0
			log.Printf("[Indexer] Нет lastTS в Redis, начнем с 0")
		} else {
			return err
		}
	}

	cursor, _ := rds.Get(ctx, "collection:cursor:"+collectionAddress).Result()
	var cursorPtr *string
	if cursor != "" {
		cursorPtr = &cursor
	}

	maxTS := lastTS
	page := 1

	for {
		log.Printf(
			"[Indexer] Загружаем страницу %d, cursor=%v",
			page, cursorPtr,
		)

		resp, err := GetCollectionHistory(collectionAddress, cursor)
		if err != nil {
			return err
		}

		if len(resp.Response.Items) == 0 {
			log.Printf("[Indexer] Пустая страница")
			break
		}

		for _, item := range resp.Response.Items {
			addr := item.Address

			// --- обновляем maxTS ---
			if item.Timestamp > maxTS {
				maxTS = item.Timestamp
			}

			switch item.TypeData.Type {

			case "mint":
				priceKey := fmt.Sprintf("nft:last_price:%s:%s", collectionAddress, addr)

				// ⚠️ ставим цену ТОЛЬКО если её нет
				ok, err := rds.SetNX(ctx, priceKey, 1.4, 0).Result()
				if err != nil {
					return err
				}
				if ok {
					log.Printf("[Indexer][mint] NFT %s — %s, price=1.4", addr, item.Name)
				} else {
					log.Printf("[Indexer][mint] NFT %s — %s, цена уже есть", addr, item.Name)
				}
				sumKey := "collection:sum:" + collectionAddress
				countKey := "collection:count:" + collectionAddress

				pipe := rds.TxPipeline()
				pipe.IncrByFloat(ctx, sumKey, 1.4)
				pipe.Incr(ctx, countKey)
				if _, err := pipe.Exec(ctx); err != nil {
					return err
				}

			case "sold":
				price, ok := extractPrice(item)
				if !ok {
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

				if oldPrice == price {
					continue
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
				
				
					saleEvent := struct {
						Address   string  `json:"address"`
						Name      string  `json:"name"`
						Price     float64 `json:"price"`
						NewOwner  string  `json:"newowner"`
						Timestamp int64   `json:"timestamp"`
					}{
						Address:   addr,
						Name:      item.Name,
						Price:     price,
						NewOwner: item.TypeData.NewOwner,
						Timestamp: item.Timestamp,
					}

					saleJSON, err := json.Marshal(saleEvent)
					if err != nil {
						log.Printf("[Indexer] error marshal sale event: %v", err)
					} else {
						if err := rds.RPush(ctx, "collection:new_sales", saleJSON).Err(); err != nil {
							log.Printf("[Indexer] error push sale to queue: %v", err)
						}
					}
				
				
				log.Printf(
					"[Indexer][sold] NFT %s — %s, old=%.4f new=%.4f",
					addr, item.Name, oldPrice, price,
				)

			}
		}

		// --- cursor ---
		if resp.Response.Cursor == "" {
			log.Printf("[Indexer] Конец истории")
			break
		}

		// обновляем cursor для следующей страницы
		cursor = resp.Response.Cursor

		// сохраняем в Redis
		rds.Set(ctx, "collection:cursor:"+collectionAddress, cursor, 0)
		if isFirst {
					rds.Set(ctx, primaryKey, "true", 0)
				}
		page++
	}

	// --- сохраняем lastTS ---
	if err := rds.Set(ctx, "collection:last_ts:"+collectionAddress, maxTS, 0).Err(); err != nil {
		return err
	}

	rds.Set(ctx, "collection:"+collectionAddress+":indexed", "true", 0)

	log.Printf("[Indexer] Индексация завершена, lastTS=%d", maxTS)

	return nil
}


type OwnerNftsResponse struct {
	Success bool `json:"success"`
	Response struct {
		Cursor *string `json:"cursor"`
		Items  []struct {
			Address           string `json:"address"`
			CollectionAddress string `json:"collectionAddress"`
			Name              string `json:"name"`
		} `json:"items"`
	} `json:"response"`
}

func GetOwnerAvgBuyPrice(
	rds *redis.Client,
	ownerAddress string,
) (avg float64, count int, err error) {

	ctx := Ctx
	var sum float64
	var total int

	collectionAddress := os.Getenv("COLLECTION_ADDRESS")
	cursor := ""

	log.Printf(
		"[OwnerAvg] owner=%s collection=%s",
		ownerAddress,
		collectionAddress,
	)

	for {
		u := fmt.Sprintf(
			"https://api.getgems.io/public-api/v1/nfts/collection/%s/owner/%s?limit=100",
			collectionAddress,
			ownerAddress,
		)
		if cursor != "" {
			u += "&after=" + url.QueryEscape(cursor)
		}

		log.Printf("[OwnerAvg] request: %s", u)

		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return 0, 0, err
		}
		req.Header.Add("accept", "application/json")
		req.Header.Add("Authorization", os.Getenv("GETGEMS_TOKEN"))

		resp, err := apiqueue.Queue.Enqueue(req, apiqueue.Low)
		if err != nil {
			return 0, 0, err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		log.Printf("[OwnerAvg] status=%d body=%s", resp.StatusCode, string(body))

		if resp.StatusCode != http.StatusOK {
			return 0, 0, fmt.Errorf("getgems error %d", resp.StatusCode)
		}

		var data OwnerNftsResponse
		if err := json.Unmarshal(body, &data); err != nil {
			return 0, 0, err
		}

		log.Printf(
			"[OwnerAvg] received %d NFTs",
			len(data.Response.Items),
		)

		for _, nft := range data.Response.Items {
			log.Printf(
				"[OwnerAvg] nft=%s name=%s",
				nft.Address,
				nft.Name,
			)

			priceKey := fmt.Sprintf("nft:last_price:%s:%s", collectionAddress, nft.Address,)
			price, err := rds.Get(ctx, priceKey).Float64()
			
			if err != nil {
				if errors.Is(err, redis.Nil) {
					log.Printf(
						"[OwnerAvg] no price in redis for nft=%s",
						nft.Address,
					)
					continue
				}
				return 0, 0, err
			}

			log.Printf(
				"[OwnerAvg] matched nft=%s price=%.4f",
				nft.Address,
				price,
			)

			sum += price
			total++
		}

		if data.Response.Cursor == nil || *data.Response.Cursor == "" {
			break
		}
		cursor = *data.Response.Cursor
	}

	if total == 0 {
		log.Printf("[OwnerAvg] NO NFT FOUND")
		return 0, 0, nil
	}

	avg = sum / float64(total)

	log.Printf(
		"[OwnerAvg] DONE count=%d avg=%.4f",
		total,
		avg,
	)

	return avg, total, nil
}
