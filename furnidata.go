package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
)

const (
	IconBaseURL      = "https://images.habbo.com/dcr/hof_furni/%s/"
	FurniDataBaseURL = "https://origins.habbo.com/gamedata/furnidata_json/6e9408e1a9015a995c15203f246d8d2d61c5f72d"
	APIItemsURL      = "https://tc-api.serversia.com/items"
)

var (
	furniData     map[string]FurniData
	externalTexts map[string]string
	apiItems      map[string]APIItem
	mu            sync.RWMutex
)

type FurniData struct {
	ID         int    `json:"id"`
	ClassName  string `json:"classname"`
	Revision   int    `json:"revision"`
	Name       string `json:"name"`
	Rare       bool   `json:"rare"`
	ExternalID string `json:"externalid"`
}

type APIItem struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Slug  string  `json:"slug"`
	HCVal float64 `json:"hc_val"`
}

var specialNameMappings = map[string]string{
	"Habbo Cola Machine":     "Cola Machine",
	"Bonnie Blonde's Pillow": "Purple Velvet Pillow",
	"Imperial Teleport":      "Imperial Teleports",
	"poster_5003":            "Purple Garland",
	"poster_5000":            "Green Garland",
	"Club sofa":              "Club Sofa",
	"Dicemaster":             "Dice Master",
}

func init() {
	furniData = make(map[string]FurniData)
	externalTexts = make(map[string]string)
	apiItems = make(map[string]APIItem)
}

func GetIconURL(classname string, itemType string, props string) string {
	mu.RLock()
	defer mu.RUnlock()

	classnameForIcon := strings.ReplaceAll(classname, "*", "_")

	var iconURL string
	if itemType == "I" {
		revision := "56783"
		iconURL = fmt.Sprintf("%s%s%s_icon.png", fmt.Sprintf(IconBaseURL, revision), "poster", props)
	} else {
		furni, ok := furniData[classname]
		if !ok {
			log.Printf("Furni data not found for classname: %s", classname)
			return ""
		}

		revision := fmt.Sprintf("%d", furni.Revision)
		iconURL = fmt.Sprintf("%s%s_icon.png", fmt.Sprintf(IconBaseURL, revision), classnameForIcon)
	}

	log.Printf("Requesting icon URL for classname %s: %s", classname, iconURL)
	return iconURL
}

func GetItemName(class string, itemType string, props string) string {
	mu.RLock()
	defer mu.RUnlock()

	var key string
	if itemType == "I" {
		key = fmt.Sprintf("poster_%s_name", props)
	} else {
		key = fmt.Sprintf("furni_%s_name", class)
	}

	name, ok := externalTexts[key]
	if ok && name != "" {
		return name
	}

	if itemType == "I" {
		return fmt.Sprintf("%s_%s", class, props)
	} else {
		return class
	}
}

func LoadFurniData(gameHost string) error {
	resp, err := http.Get(FurniDataBaseURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var data struct {
		RoomItemTypes struct {
			FurniType []FurniData `json:"furnitype"`
		} `json:"roomitemtypes"`
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	furniData = make(map[string]FurniData)
	for _, furni := range data.RoomItemTypes.FurniType {
		furniData[furni.ClassName] = furni
	}

	log.Printf("Furni data map: %+v", furniData)
	return nil
}

func LoadExternalTexts(gameHost string) error {
	externalTextsURL := fmt.Sprintf("https://origins-gamedata.habbo.com/external_texts/1")

	resp, err := http.Get(externalTextsURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	lines := strings.Split(string(body), "\n")
	mu.Lock()
	defer mu.Unlock()
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			externalTexts[key] = value
		}
	}

	log.Printf("Loaded %d external texts", len(externalTexts))
	return nil
}

func LoadAPIItems() error {
	resp, err := http.Get(APIItemsURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var items []APIItem
	err = json.NewDecoder(resp.Body).Decode(&items)
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	apiItems = make(map[string]APIItem)
	for _, item := range items {
		apiItems[item.Name] = item
	}

	log.Printf("Loaded %d API items", len(apiItems))
	return nil
}

func GetHCValue(itemName string) float64 {
	mu.RLock()
	defer mu.RUnlock()

	// Check for special mappings
	if mappedName, ok := specialNameMappings[itemName]; ok {
		itemName = mappedName
	}

	if item, ok := apiItems[itemName]; ok {
		return item.HCVal
	}

	log.Printf("No HC value found for item name: %s", itemName)
	return 0
}
