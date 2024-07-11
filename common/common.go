package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/room"
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

type EnrichedInventoryItem struct {
	inventory.Item
	Name    string
	IconURL string
	HCValue float64
}

type EnrichedRoomObject struct {
	room.Object
	Name      string
	IconURL   string
	HCValue   float64
	Width     int
	Height    int
	X         int
	Y         int
	Direction int
}

type EnrichedRoomItem struct {
	room.Item
	Name     string
	IconURL  string
	HCValue  float64
	Location string
}

func GetItemName(class string, itemType string, props string) string {
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

func GetIconURL(classname string, itemType string, props string) string {
	classnameForIcon := strings.ReplaceAll(classname, "*", "_")

	var iconURL string
	if itemType == "I" {
		revision := "56783"
		iconURL = fmt.Sprintf("%s%s%s_icon.png", fmt.Sprintf(IconBaseURL, revision), "poster", props)
	} else {
		furni, ok := furniData[classname]
		if !ok {
			return ""
		}

		revision := fmt.Sprintf("%d", furni.Revision)
		iconURL = fmt.Sprintf("%s%s_icon.png", fmt.Sprintf(IconBaseURL, revision), classnameForIcon)
	}

	return iconURL
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

func GetHCValue(itemName string) float64 {
	if mappedName, exists := specialNameMappings[itemName]; exists {
		itemName = mappedName
	}

	if item, ok := apiItems[itemName]; ok {
		return item.HCVal
	}

	return 0
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

	furniData = make(map[string]FurniData)
	for _, furni := range data.RoomItemTypes.FurniType {
		furniData[furni.ClassName] = furni
	}

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
	externalTexts = make(map[string]string)
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			externalTexts[key] = value
		}
	}

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

	apiItems = make(map[string]APIItem)
	for _, item := range items {
		apiItems[item.Name] = item
	}

	return nil
}

func EnrichInventoryItem(item inventory.Item) EnrichedInventoryItem {
	return EnrichedInventoryItem{
		Item:    item,
		Name:    GetItemName(item.Class, string(item.Type), item.Props),
		IconURL: GetIconURL(item.Class, string(item.Type), item.Props),
		HCValue: GetHCValue(GetItemName(item.Class, string(item.Type), item.Props)),
	}
}

func EnrichRoomObject(obj room.Object) EnrichedRoomObject {
	return EnrichedRoomObject{
		Object:    obj,
		Name:      GetItemName(obj.Class, "S", ""),
		IconURL:   GetIconURL(obj.Class, "S", ""),
		HCValue:   GetHCValue(GetItemName(obj.Class, "S", "")),
		Width:     obj.Width,
		Height:    obj.Height,
		X:         obj.X,
		Y:         obj.Y,
		Direction: obj.Direction,
	}
}

func EnrichRoomItem(item room.Item) EnrichedRoomItem {
	return EnrichedRoomItem{
		Item:     item,
		Name:     GetItemName(item.Class, "I", item.Type),
		IconURL:  GetIconURL(item.Class, "I", item.Type),
		HCValue:  GetHCValue(GetItemName(item.Class, "I", item.Type)),
		Location: item.Location,
	}
}

func GetInventorySummary(items map[int]inventory.Item) string {
	itemCounts := make(map[string]int)
	totalHC := 0.0

	for _, item := range items {
		name := GetItemName(item.Class, string(item.Type), item.Props)
		itemCounts[name]++
		totalHC += GetHCValue(name)
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Total unique items: %d\n", len(itemCounts)))
	summary.WriteString(fmt.Sprintf("Total items: %d\n", len(items)))
	summary.WriteString(fmt.Sprintf("Total wealth: %.2f HC (values from traderclub.gg)\n", totalHC))
	summary.WriteString("------------------\n")

	for name, count := range itemCounts {
		hcValue := GetHCValue(name)
		summary.WriteString(fmt.Sprintf("%s: %d (%.2f HC)\n", name, count, hcValue))
	}

	return summary.String()
}

func GetRoomSummary(objects map[int]room.Object, items map[int]room.Item) string {
	itemCounts := make(map[string]int)
	totalHC := 0.0

	for _, obj := range objects {
		name := GetItemName(obj.Class, "S", "")
		itemCounts[name]++
		totalHC += GetHCValue(name)
	}

	for _, item := range items {
		name := GetItemName(item.Class, "I", item.Type)
		itemCounts[name]++
		totalHC += GetHCValue(name)
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Total unique items: %d\n", len(itemCounts)))
	summary.WriteString(fmt.Sprintf("Total items: %d\n", len(objects)+len(items)))
	summary.WriteString(fmt.Sprintf("Total wealth: %.2f HC (values from traderclub.gg)\n", totalHC))
	summary.WriteString("------------------\n")

	for name, count := range itemCounts {
		hcValue := GetHCValue(name)
		summary.WriteString(fmt.Sprintf("%s: %d (%.2f HC)\n", name, count, hcValue))
	}

	return summary.String()
}

func GetInventoryItemDetails(item inventory.Item) string {
	name := GetItemName(item.Class, string(item.Type), item.Props)
	return fmt.Sprintf("Name: %s\nID: %d\nType: %s\nClass: %s\nProps: %s\n",
		name, item.ItemId, item.Type, item.Class, item.Props)
}

func GetRoomItemDetails(item room.Item) string {
	name := GetItemName(item.Class, "I", item.Type)
	return fmt.Sprintf("Name: %s\nID: %d\nClass: %s\nOwner: %s\nLocation: %s\nType: %s\n",
		name, item.Id, item.Class, item.Owner, item.Location, item.Type)
}

func GetRoomObjectDetails(obj room.Object) string {
	name := GetItemName(obj.Class, "S", "")
	return fmt.Sprintf("Name: %s\nID: %d\nClass: %s\nPosition: (%d, %d, %.2f)\nSize: %dx%d\nDirection: %d\n",
		name, obj.Id, obj.Class, obj.X, obj.Y, obj.Z, obj.Width, obj.Height, obj.Direction)
}
