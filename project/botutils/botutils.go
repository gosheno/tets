package botutils

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	apiqueue "tg-getgems-bot/api"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/sync/singleflight"
	"gopkg.in/telebot.v3"
)

func loadDefaultFont() font.Face {
	return basicfont.Face7x13
}

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

var requestGroup singleflight.Group

// --- Утилиты ---

// fetchJSON делает HTTP GET и парсит JSON в result
func fetchJSON(url string, result any) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("accept", "application/json")
	if token := os.Getenv("GETGEMS_TOKEN"); token != "" {
		req.Header.Add("Authorization", token)
	}

	resp, err := apiqueue.Queue.Enqueue(req, apiqueue.Low)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("status %s", resp.Status)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return body, err
	}

	return body, nil
}

// timeRange возвращает minTime и maxTime в миллисекундах для n дней назад
func timeRange(days int) (min, max int64) {
	max = time.Now().UnixMilli()
	min = max - int64(days)*24*3600*1000
	return
}

// minFloat возвращает минимальное из двух float64
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// calcProfit возвращает процент прибыли
func calcProfit(price, base float64) float64 {
	return (price - base) / base * 100
}

// --- API-функции ---

// GetMinPrice возвращает минимальную цену Reactor из коллекции с кэшированием
func GetMinPrice(redisClient *redis.Client) (float64, error) {
	cacheKey := "min_price_reactor"

	val, err, _ := requestGroup.Do(cacheKey, func() (interface{}, error) {
		cached, err := redisClient.Get(Ctx, cacheKey).Result()
		if err == nil && cached != "" {
			price, _ := strconv.ParseFloat(cached, 64)
			log.Printf("[Redis] Возврат из кэша min_price_reactor: %.2f", price)
			return price, nil
		}

		type ApiResponse struct {
			Response struct {
				Attributes []struct {
					Values []struct {
						Value    string `json:"value"`
						MinPrice string `json:"minPrice"`
					} `json:"values"`
				} `json:"attributes"`
			} `json:"response"`
		}

		var data ApiResponse
		url := "https://api.getgems.io/public-api/v1/collection/attributes/EQC4XEulxb05Le5gF6esMtDWT5XZ6tlzlMBQGNsqffxpdC5U"
		if _, err := fetchJSON(url, &data); err != nil {
			return 0.0, err
		}

		for _, attr := range data.Response.Attributes {
			for _, v := range attr.Values {
				if v.Value == "Reactor" {
					price, _ := strconv.ParseFloat(v.MinPrice, 64)
					redisClient.Set(Ctx, cacheKey, price, time.Hour)
					log.Printf("[API] min_price_reactor: %.2f", price)
					return price, nil
				}
			}
		}
		return 0.0, errors.New("не найден min_price Reactor")
	})

	if err != nil {
		return 0, err
	}
	return val.(float64), nil
}

// GetMinPriceGreen возвращает минимальный флор Green с кэшированием
func GetMinPriceGreen(redisClient *redis.Client) (float64, error) {
	cacheKey := "min_price_green"
	val, err, _ := requestGroup.Do(cacheKey, func() (interface{}, error) {
		cached, _ := redisClient.Get(Ctx, cacheKey).Result()
		if cached != "" {
			price, _ := strconv.ParseFloat(cached, 64)
			log.Printf("[Redis] min_price_green: %.2f", price)
			return price, nil
		}

		type ApiResp struct {
			Response struct {
				FloorPrice float64 `json:"floorPrice"`
			} `json:"response"`
		}
		var data ApiResp
		url := "https://api.getgems.io/public-api/v1/collection/stats/EQAnmo8tBH8gSErzWDrdlJiF8kxgfJEynKMIBxL2MkuHvPBc"
		if _, err := fetchJSON(url, &data); err != nil {
			return 0.0, err
		}
		redisClient.Set(Ctx, cacheKey, data.Response.FloorPrice, 5*time.Hour)
		log.Printf("[API] min_price_green: %.2f", data.Response.FloorPrice)
		return data.Response.FloorPrice, nil
	})
	if err != nil {
		return 0, err
	}
	return val.(float64), nil
}

// GetMinPriceFloor возвращает минимальный флор коллекции с кэшированием
func GetMinPriceFloor(redisClient *redis.Client) (float64, error) {
	cacheKey := "min_price_floor"
	val, err, _ := requestGroup.Do(cacheKey, func() (interface{}, error) {
		cached, _ := redisClient.Get(Ctx, cacheKey).Result()
		if cached != "" {
			price, _ := strconv.ParseFloat(cached, 64)
			log.Printf("[Redis] min_price_floor: %.2f", price)
			return price, nil
		}

		type ApiResp struct {
			Response struct {
				FloorPrice float64 `json:"floorPrice"`
			} `json:"response"`
		}
		var data ApiResp
		url := "https://api.getgems.io/public-api/v1/collection/stats/EQC4XEulxb05Le5gF6esMtDWT5XZ6tlzlMBQGNsqffxpdC5U"
		if _, err := fetchJSON(url, &data); err != nil {
			return 0.0, err
		}
		redisClient.Set(Ctx, cacheKey, data.Response.FloorPrice, 5*time.Hour)
		log.Printf("[API] min_price_floor: %.2f", data.Response.FloorPrice)
		return data.Response.FloorPrice, nil
	})
	if err != nil {
		return 0, err
	}
	return val.(float64), nil
}

// GetTonPrice возвращает текущую цену TON в USD
func GetTonPrice(redisClient *redis.Client) (float64, error) {
	cacheKey := "ton_usd"
	val, err, _ := requestGroup.Do(cacheKey, func() (interface{}, error) {
	cached, _ := redisClient.Get(Ctx, cacheKey).Result()
		if cached != "" {
			price, _ := strconv.ParseFloat(cached, 64)
			return price, nil
		}
		type quote struct {
			Price float64 `json:"price"`
		}
		var parsed struct {
			Quotes map[string]quote `json:"quotes"`
		}
		url := "https://api.coinpaprika.com/v1/tickers/ton-toncoin"
		body, err := fetchJSON(url, &parsed)
		if err != nil {
			return 0.0, err
		}
		q, ok := parsed.Quotes["USD"]
		if !ok {
			return 0.0, fmt.Errorf("USD quote missing: %s", string(body))
		}
		redisClient.Set(Ctx, cacheKey, q.Price, 5*time.Minute)
		return q.Price, nil
	})
	if err != nil {
		return 0, err
	}
	return val.(float64), nil
}

// GetFirstOnSalePrice возвращает цену первой NFT на продаже
func GetFirstOnSalePrice(redisClient *redis.Client) (float64, error) {
	cacheKey := "first_price_collection"
	val, err, _ := requestGroup.Do(cacheKey, func() (interface{}, error) {
		cached, _ := redisClient.Get(Ctx, cacheKey).Result()
		if cached != "" {
			price, _ := strconv.ParseFloat(cached, 64)
			log.Printf("[Redis] first_price_collection: %.2f", price)
			return price, nil
		}

		type OnSaleResponse struct {
			Response struct {
				Items []struct {
					Sale struct {
						FullPrice string `json:"fullPrice"`
					} `json:"sale"`
				} `json:"items"`
			} `json:"response"`
		}
		var data OnSaleResponse
		url := "https://api.getgems.io/public-api/v1/nfts/offchain/on-sale/EQC4XEulxb05Le5gF6esMtDWT5XZ6tlzlMBQGNsqffxpdC5U"
		if _, err := fetchJSON(url, &data); err != nil {
			return 0.0, err
		}
		if len(data.Response.Items) == 0 {
			return 0.0, errors.New("нет NFT в продаже")
		}
		price, _ := strconv.ParseFloat(data.Response.Items[0].Sale.FullPrice, 64)
		priceFinal := price / 1e9
		redisClient.Set(Ctx, cacheKey, priceFinal, time.Hour)
		log.Printf("[API] first_price_collection: %.2f", priceFinal)
		return priceFinal, nil
	})
	if err != nil {
		return 0, err
	}
	return val.(float64), nil
}

// GetCount возвращает количество купленных фрагментов за день/неделю/месяц
func GetCount(redisClient *redis.Client) (*FragmentCount, error) {
	now := time.Now().UTC()
	dayKey := "collection:sales:day:" + now.Format("20060102")

	year, week := now.ISOWeek()
	weekKey := fmt.Sprintf("collection:sales:week:%d%02d", year, week)

	monthKey := "collection:sales:month:" + now.Format("200601")

	get := func(key string) int {
		v, err := redisClient.Get(Ctx, key).Int()
		if err != nil {
			return 0
		}
		return v
	}

	count := &FragmentCount{
		Day:   get(dayKey),
		Week:  get(weekKey),
		Month: get(monthKey),
	}

	log.Printf(
		"[Redis] fragment_count: день=%d, неделя=%d, месяц=%d",
		count.Day, count.Week, count.Month,
	)

	return count, nil
}

// fetchHistoryCount подсчитывает количество продаж в истории
func fetchHistoryCount(url string) (int, error) {
	type HistoryResponse struct {
		Success  bool `json:"success"`
		Response struct {
			Items []struct {
				EventType string `json:"eventType"`
			} `json:"items"`
		} `json:"response"`
	}
	var data HistoryResponse
	body, err := fetchJSON(url, &data)
	if err != nil {
		return 0, err
	}
	count := len(data.Response.Items)
	if count == 0 {
		log.Printf("[API] fetchHistoryCount пусто: %s", string(body))
	}
	return count, nil
}

// GenerateStatImage создает картинку со статистикой цен и покупок
//
//go:embed Alkia.ttf
var ttfBytes []byte


type FragmentCount struct {
	Day, Week, Month int
}
func GenerateStatImage(
	price, startProfit, priceG, endProfit, avgPrice, avgProfit float64,
	count *FragmentCount, TonPrice float64, startProfitUsd float64,
) (string, error) {

	const (
		width     = 800
		height    = 700
		margin    = 20
		numBlocks = 4
		fontSize  = 32
	)
	
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// --- цвета ---
	bgColor := color.RGBA{245, 245, 245, 255}
	blockColors := []color.RGBA{
		{230, 230, 250, 255},
		{210, 210, 240, 255},
	}
	textColor := color.RGBA{0, 0, 0, 255}
	profitGoodColor := color.RGBA{0, 150, 0, 255}
	profitBadColor := color.RGBA{200, 0, 0, 255}

	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// --- шрифт ---
	ttfFont, err := opentype.Parse(ttfBytes)
	if err != nil {
		return "", err
	}

	fontFace, err := opentype.NewFace(ttfFont, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return "", err
	}
	smallFontSize := float64(fontSize) * 0.55

	smallFontFace, err := opentype.NewFace(ttfFont, &opentype.FaceOptions{
		Size:    smallFontSize,
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return "", err
	}
	drawTextSmall := func(x, y int, text string, c color.Color) {
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(c),
			Face: smallFontFace,
			Dot:  fixed.P(x, y),
		}
		d.DrawString(text)
	}
	drawText := func(x, y int, text string, c color.Color) {
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(c),
			Face: fontFace,
			Dot:  fixed.P(x, y),
		}
		d.DrawString(text)
	}

	// --- рисование текста с подсветкой чисел ---
	drawTextColoredDigits := func(
		x, y int,
		text string,
		baseColor color.Color,
		good color.Color,
		bad color.Color,
	) {
		currX := x
		var buf strings.Builder

		flushNumber := func() {
			if buf.Len() == 0 {
				return
			}

			raw := buf.String()
			numStr := strings.TrimSuffix(raw, "%")
			val, err := strconv.ParseFloat(numStr, 64)

			colorToUse := baseColor
			if err == nil {
				colorToUse = getProfitColor(val, good, bad)
			}

			for _, r := range raw {
				drawText(currX, y, string(r), colorToUse)
				advance, ok := fontFace.GlyphAdvance(r)
				if !ok {
					advance = fontFace.Metrics().Height
				}
				currX += advance.Ceil()
			}

			buf.Reset()
		}

		for _, r := range text {
			if (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '%' {
				buf.WriteRune(r)
				continue
			}

			flushNumber()

			drawText(currX, y, string(r), baseColor)
			advance, ok := fontFace.GlyphAdvance(r)
			if !ok {
				advance = fontFace.Metrics().Height
			}
			currX += advance.Ceil()
		}

		flushNumber()
	}

	measure := func(text string) int {
		d := &font.Drawer{Face: fontFace}
		return d.MeasureString(text).Ceil()
	}

	blockHeight := (height - 2*margin) / numBlocks

	// --- БЛОКИ ---
	blocks := []struct {
		title string
		draw  func(yStart int)
	}{
		{
			title: "Heart Locket Floor",
			draw: func(y int) {
				val := fmt.Sprintf("%.2f", price)
				x := width/2 - measure(val)/2
				drawTextColoredDigits(x, y+blockHeight/2, val, textColor, profitGoodColor, profitBadColor)
			},
		},
		{
			title: "Stats (secondary market)",
			draw: func(y int) {
				priceusd := 1.4*3.125

				leftTitle  := fmt.Sprintf("Mint: 1.4      (%.2f$)   ", priceusd)
				leftProfit := fmt.Sprintf("PnL: %.2f%% (%.2f%%)", startProfit, startProfitUsd)

				rightTitle := fmt.Sprintf("Actual: %.2f (%.2f$)", priceG, priceG*TonPrice)
				rightProfit := fmt.Sprintf("PnL: %.2f%%", endProfit)

				drawTextColoredDigits(
					width/4-measure(leftTitle)/2,
					y+blockHeight/2,
					leftTitle,
					textColor,
					profitGoodColor,
					profitBadColor,
				)

				drawTextColoredDigits(
					width/4-measure(leftProfit)/2,
					y+blockHeight/2+fontSize*3/2,
					leftProfit,
					textColor,
					profitGoodColor,
					profitBadColor,
				)

				drawTextColoredDigits(
					3*width/4-measure(rightTitle)/2,
					y+blockHeight/2,
					rightTitle,
					textColor,
					profitGoodColor,
					profitBadColor,
				)

				drawTextColoredDigits(
					3*width/4-measure(rightProfit)/2,
					y+blockHeight/2+fontSize*3/2,
					rightProfit,
					textColor,
					profitGoodColor,
					profitBadColor,
				)
			},
		},
		{
			title: "Trades",
			draw: func(y int) {
				lines := []string{
					fmt.Sprintf("24h: %d", count.Day),
					fmt.Sprintf("Week: %d", count.Week),
					fmt.Sprintf("Month: %d", count.Month),
				}

				for i, l := range lines {
					x := width/4*(i+1) - measure(l)/2
					drawText(x, y+blockHeight/2, l, textColor)
				}
			},
		},
		{
			title: "Community Stats (owned NFTs)",
			draw: func(y int) {
				line1 := fmt.Sprintf("Avg price: %.2f", avgPrice)
				line2 := fmt.Sprintf("PnL: %.2f%%", avgProfit)
				subLine := "Current total community profit from the sale voting" // ← твой текст
				drawTextColoredDigits(
					width/2-measure(line1)/2,
					y+blockHeight/2,
					line1,
					textColor,
					profitGoodColor,
					profitBadColor,
				)
				drawTextColoredDigits(
					width/2-measure(line2)/2,
					y+blockHeight/2+fontSize*3/2,
					line2,
					textColor,
					profitGoodColor,
					profitBadColor,
				)
				drawTextSmall(
					width/2-measure(subLine)/2,
					y+blockHeight/2+fontSize*3/2+int(fontSize),
					subLine,
					color.RGBA{90, 90, 90, 255},
				)
			},
		},
	}

	// --- РЕНДЕР ---
	for i, b := range blocks {
		y := margin + i*blockHeight

		draw.Draw(
			img,
			image.Rect(margin, y, width-margin, y+blockHeight),
			&image.Uniform{blockColors[i%2]},
			image.Point{},
			draw.Src,
		)

		drawText(
			width/2-measure(b.title)/2,
			y+fontSize,
			b.title,
			textColor,
		)

		b.draw(y)
	}

	// --- save ---
	tmpFile := "/tmp/stat_image.png"
	f, err := os.Create(tmpFile)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return "", err
	}

	return tmpFile, nil
}

func getProfitColor(p float64, good, bad color.Color) color.Color {
	if p >= 0 {
		return good
	}
	return bad
}

// HandleLook выводит ID чата и ID ветки в консоль
func HandleLook(c telebot.Context) error  {
	chatID := c.Chat().ID
	threadID := c.Message().ThreadID

	fmt.Printf("[/look] Chat ID: %d | Thread ID: %d\n", chatID, threadID)
	fmt.Printf("[/look] Chat ID: %d\n[/look] Thread ID: %d\n", chatID, threadID)
	return nil
}

// HandlePS возвращает текущий статус бота
func HandlePS(redisClient *redis.Client, c telebot.Context) error {
	status := "✅ Бот работает нормально\n\n"
	collectingStatus, _ := GetValue(redisClient, "process:collecting")
	status += "• статус: " + collectingStatus + "\n"
	c.Send(status, &telebot.SendOptions{ThreadID: c.Message().ThreadID})
	return c.Send(status, &telebot.SendOptions{ThreadID: c.Message().ThreadID})
	
}

// getProcessStatus возвращает статус процесса из Redis
func getProcessStatus(redisClient *redis.Client, processName string) string {
	status, err := GetValue(redisClient, "process:"+processName)
	if err != nil || status == "" {
		return "⏳ В ожидании"
	}

	// Проверяем если это прогресс (формат: running:50%)
	if len(status) > 8 && status[:8] == "running:" {
		percentage := status[8:]
		return "🔄 В процессе: " + percentage
	}

	switch status {
	case "running":
		return "🔄 В процессе"
	case "idle":
		return "⏳ В ожидании"
	case "error":
		return "❌ Ошибка"
	default:
		return "❓ " + status
	}
}

func parseChatID(s string) int64 {
	var id int64
	fmt.Sscan(s, &id)
	return id
}

func parseTreadID(s string) int {
	var id int
	fmt.Sscan(s, &id)
	return id
}

func NotifyNewSales(bot *telebot.Bot, redisClient *redis.Client, collection string) {
	ctx := context.Background()
	for {
		// Проверяем очередь новых продаж
		saleJSON, err := redisClient.LPop(ctx, "collection:new_sales").Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				time.Sleep(10 * time.Second) // очередь пустая
				continue
			}
			log.Printf("[Notifier] Redis error: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		var sale struct {
			Address   string  `json:"address"`
			Name      string  `json:"name"`
			Price     float64 `json:"price"`
			Timestamp int64   `json:"timestamp"`
		}

		if err := json.Unmarshal([]byte(saleJSON), &sale); err != nil {
			log.Printf("[Notifier] Ошибка парсинга saleJSON: %v", err)
			continue
		}

		// --- Отправляем уведомление ---
		adminID := os.Getenv("CHAT_ID")
		if adminID == "" {
			continue
		}
		chat := &telebot.Chat{ID: parseChatID(adminID)}
		msgText := fmt.Sprintf(
			"💎 Новая покупка — %s\nЦена: %.4f TON\nВремя: %s",
			sale.Name,
			sale.Price,
			time.UnixMilli(sale.Timestamp).Format("02 Jan 2006 15:04:05"),
		)

		if _, err := bot.Send(chat, msgText); err != nil {
			log.Printf("[Notifier] Ошибка отправки уведомления: %v", err)
		} else {
			log.Printf("[Notifier] Отправлено уведомление о покупке NFT %s", sale.Address)
		}
	}
}
