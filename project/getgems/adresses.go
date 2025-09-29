package getgems

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/xssnick/tonutils-go/address"
)

type TonNftItem struct {
	Address string `json:"address"` // raw формат: 0:HEX
}

type TonCollectionResponse struct {
	NFTs  []TonNftItem `json:"nft_items"`
	Total int          `json:"total"`
}

func getTonCollectionNFTs(collectionAddr string) ([]string, error) {
	var all []string
	offset := 0
	limit := 1000

	for {
		url := fmt.Sprintf(
			"https://tonapi.io/v2/nfts/collections/%s/items?limit=%d&offset=%d",
			collectionAddr, limit, offset,
		)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		if token := os.Getenv("TONAPI_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("status %s: %s", resp.Status, string(body))
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var res TonCollectionResponse
		if err := json.Unmarshal(bodyBytes, &res); err != nil {
			return nil, err
		}

		for _, item := range res.NFTs {
			addr, err := address.ParseRawAddr(item.Address)
			if err != nil {
				log.Println("⚠️ Ошибка конвертации адреса:", item.Address, err)
				continue
			}
			all = append(all, addr.String())
		}

		log.Printf("📦 Offset %d: загружено %d NFT (всего %d)", offset, len(res.NFTs), len(all))

		if len(res.NFTs) < limit {
			break
		}
		offset += limit
	}

	return all, nil
}

func mainaddr() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env файл не найден, используем переменные окружения")
	}

	collection := "EQAnmo8tBH8gSErzWDrdlJiF8kxgfJEynKMIBxL2MkuHvPBc"
	nfts, err := getTonCollectionNFTs(collection)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✅ Всего NFT: %d\n", len(nfts))

	// Открываем файл для записи
	file, err := os.Create("nft_addresses.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Записываем каждый адрес в файл
	for _, addr := range nfts {
	_, err := file.WriteString(addr + "\n")
	if err != nil {
		log.Fatal(err)
	}
}
	fmt.Println("📁 Адреса сохранены в nft_addresses.txt")
}
