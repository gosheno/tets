package botutils

import (
	_ "embed"
	"encoding/json"
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
	apiqueue "tg-getgems-bot/api"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
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
		log.Println("❌ Ошибка создания запроса:", err)
		return 0, nil, err
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", os.Getenv("GETGEMS_TOKEN"))
	resp, err := apiqueue.Queue.Enqueue(req, apiqueue.Low)
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
	fmt.Println("[API] Запрос min_price_green")

	url := "https://api.getgems.io/public-api/v1/collection/stats/EQAnmo8tBH8gSErzWDrdlJiF8kxgfJEynKMIBxL2MkuHvPBc"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", os.Getenv("GETGEMS_TOKEN"))
	resp, err := apiqueue.Queue.Enqueue(req, apiqueue.Low)
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
	return data.Response.FloorPrice, bodyBytes, nil
}
func GetMinPriceFloor(redisClient *redis.Client) (float64, []byte, error) {
	url := "https://api.getgems.io/public-api/v1/collection/stats/EQC4XEulxb05Le5gF6esMtDWT5XZ6tlzlMBQGNsqffxpdC5U"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", os.Getenv("GETGEMS_TOKEN"))
	resp, err := apiqueue.Queue.Enqueue(req, apiqueue.Low)
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
	fmt.Print("[API] min_price_floor: ", data.Response.FloorPrice, "\n")
	return data.Response.FloorPrice, bodyBytes, nil
}

func GetFirstOnSalePrice(redisClient *redis.Client) (float64, []byte, error) {
	cacheKey := "first_price_collection"
	cached, err := GetValue(redisClient, cacheKey)
	if err == nil && cached != "" {
		price, err := strconv.ParseFloat(cached, 64)
		if err == nil {
			fmt.Println("[Redis] Возврат из кеша first_price_collection:", price)
			return price, []byte(fmt.Sprintf("{\"cached\":true,\"price\":%f}", price)), nil
		}
	}

	url := "https://api.getgems.io/public-api/v1/nfts/offchain/on-sale/EQC4XEulxb05Le5gF6esMtDWT5XZ6tlzlMBQGNsqffxpdC5U"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("❌ Ошибка создания запроса:", err)
		return 0, nil, err
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", os.Getenv("GETGEMS_TOKEN"))

	resp, err := apiqueue.Queue.Enqueue(req, apiqueue.Low)
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

	// структура ответа (упрощённая)
	type OnSaleResponse struct {
		Response struct {
			Items []struct {
				Address string `json:"address"`
				Name    string `json:"name"`
				Sale    struct {
					Type           string `json:"type"`
					FullPrice      string `json:"fullPrice"`
					Currency       string `json:"currency"`
					MarketplaceFee string `json:"marketplaceFee"`
				} `json:"sale"`
			} `json:"items"`
		} `json:"response"`
	}

	var data OnSaleResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return 0, bodyBytes, err
	}

	if len(data.Response.Items) == 0 {
		return 0, bodyBytes, fmt.Errorf("нет NFT в продаже")
	}
	priceStr := data.Response.Items[0].Sale.FullPrice
	price, err := strconv.ParseFloat(priceStr, 64)
	pricefinal := price / 1e9
	if err != nil {
		return 0, bodyBytes, err
	}
	if err != nil {
		return 0, bodyBytes, err
	}

	fmt.Println("[API] first_price_collection:", pricefinal)
	redisClient.Set(Ctx, cacheKey, pricefinal, 3600*1_000_000_000)

	return pricefinal, bodyBytes, nil
}

// FragmentCount содержит количество купленных фрагментов за разные периоды

// GetCount возвращает количество купленных фрагментов за день, неделю и месяц
func GetCount(redisClient *redis.Client) (*FragmentCount, error) {
	count := &FragmentCount{}
	now := time.Now().Unix() * 1000             // конвертируем в миллисекунды
	dayAgo := (now - (24 * 3600 * 1000))        // 1 день назад
	weekAgo := (now - (7 * 24 * 3600 * 1000))   // 7 дней назад
	monthAgo := (now - (30 * 24 * 3600 * 1000)) // 30 дней назад

	// Адрес коллекции зеленых кусочков
	collectionAddress := "EQAnmo8tBH8gSErzWDrdlJiF8kxgfJEynKMIBxL2MkuHvPBc"

	// Получаем продажи за месяц
	monthURL := fmt.Sprintf("https://api.getgems.io/public-api/v1/collection/history/%s?minTime=%d&maxTime=%d&types=sold&limit=100",
		collectionAddress, monthAgo, now)
	monthCount, err := fetchHistoryCount(monthURL)
	if err != nil {
		log.Println("❌ Ошибка получения истории месяца:", err)
		return nil, err
	}
	count.Month = monthCount

	// Получаем продажи за неделю
	weekURL := fmt.Sprintf("https://api.getgems.io/public-api/v1/collection/history/%s?minTime=%d&maxTime=%d&types=sold&limit=100",
		collectionAddress, weekAgo, now)
	weekCount, err := fetchHistoryCount(weekURL)
	if err != nil {
		log.Println("❌ Ошибка получения истории недели:", err)
		return nil, err
	}
	count.Week = weekCount

	// Получаем продажи за день
	dayURL := fmt.Sprintf("https://api.getgems.io/public-api/v1/collection/history/%s?minTime=%d&maxTime=%d&types=sold&limit=100",
		collectionAddress, dayAgo, now)
	dayCount, err := fetchHistoryCount(dayURL)
	if err != nil {
		log.Println("❌ Ошибка получения истории дня:", err)
		return nil, err
	}
	count.Day = dayCount

	fmt.Printf("[API] fragment_count: день=%d, неделя=%d, месяц=%d\n", count.Day, count.Week, count.Month)
	return count, nil
}

// fetchHistoryCount получает и подсчитывает количество продаж из истории коллекции
func fetchHistoryCount(url string) (int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("❌ Ошибка создания запроса:", err)
		return 0, err
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", os.Getenv("GETGEMS_TOKEN"))

	resp, err := apiqueue.Queue.Enqueue(req, apiqueue.Low)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// Структура для парсинга ответа API истории коллекции
	type HistoryResponse struct {
		Success  bool `json:"success"`
		Response struct {
			Items []struct {
				EventType string `json:"eventType"`
			} `json:"items"`
		} `json:"response"`
	}

	var data HistoryResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		log.Println("❌ Ошибка парсинга JSON:", err)
		return 0, err
	}

	// Подсчитываем количество продаж (sold события)
	count := len(data.Response.Items)
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
	count *FragmentCount,
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

	drawText := func(x, y int, text string, c color.Color) {
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(c),
			Face: fontFace,
			Dot:  fixed.P(x, y),
		}
		d.DrawString(text)
	}

	drawTextColoredDigits := func(x, y int, text string, baseColor, digitColor color.Color) {
		currX := x
		for _, r := range text {
			c := baseColor
			if (r >= '0' && r <= '9') || r == '.' || r == '%' {
				c = digitColor
			}

			drawText(currX, y, string(r), c)

			advance, ok := fontFace.GlyphAdvance(r)
			if !ok {
				advance = fontFace.Metrics().Height
			}
			currX += advance.Ceil()
		}
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
				drawTextColoredDigits(x, y+blockHeight/2, val, textColor, textColor)
			},
		},
		{
			title: "Stats(secondary market)",
			draw: func(y int) {
				leftTitle := "Mint 1.4"
				leftProfit := fmt.Sprintf("Profit: %.2f%%", startProfit)

				rightTitle := fmt.Sprintf("Actual: %.2f", priceG)
				rightProfit := fmt.Sprintf("Profit: %.2f%%", endProfit)

				drawTextColoredDigits(
					width/4-measure(leftTitle)/2,
					y+blockHeight/2,
					leftTitle,
					textColor,
					getProfitColor(startProfit, profitGoodColor, profitBadColor),
				)

				drawTextColoredDigits(
					width/4-measure(leftTitle)/2,
					y+blockHeight/2+fontSize*1.5,
					leftProfit,
					textColor,
					getProfitColor(startProfit, profitGoodColor, profitBadColor),
				)

				drawTextColoredDigits(
					3*width/4-measure(rightTitle)/2,
					y+blockHeight/2,
					rightTitle,
					textColor,
					getProfitColor(endProfit, profitGoodColor, profitBadColor),
				)

				drawTextColoredDigits(
					3*width/4-measure(rightTitle)/2,
					y+blockHeight/2+fontSize*1.5,
					rightProfit,
					textColor,
					getProfitColor(endProfit, profitGoodColor, profitBadColor),
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
			title: "Community Stats(owned NFTs)",
			draw: func(y int) {
				line1 := fmt.Sprintf("Avg price: %.2f", avgPrice)
				line2 := fmt.Sprintf("Profit: %.2f%%", avgProfit)

				drawTextColoredDigits(
					width/2-measure(line1)/2,
					y+blockHeight/2,
					line1,
					textColor,
					getProfitColor(avgProfit, profitGoodColor, profitBadColor),
				)

				drawTextColoredDigits(
					width/2-measure(line1)/2,
					y+blockHeight/2+fontSize*1.5,
					line2,
					textColor,
					getProfitColor(avgProfit, profitGoodColor, profitBadColor),
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

		// title
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
func HandleLook(c telebot.Context) {
	chatID := c.Chat().ID
	threadID := c.Message().ThreadID

	fmt.Printf("[/look] Chat ID: %d | Thread ID: %d\n", chatID, threadID)
	fmt.Printf("[/look] Chat ID: %d\n[/look] Thread ID: %d\n", chatID, threadID)
}

// HandlePS возвращает текущий статус бота
func HandlePS(redisClient *redis.Client, c telebot.Context) string {
	status := "✅ Бот работает нормально\n\n"
	collectingStatus, _ := GetValue(redisClient, "process:collecting")
	status += "• статус: " + collectingStatus + "\n"
	c.Send(status, &telebot.SendOptions{ThreadID: c.Message().ThreadID})
	return status
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
