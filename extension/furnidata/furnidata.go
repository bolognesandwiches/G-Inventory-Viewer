package furnidata

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type FurniItem struct {
	ID          int    `json:"id"`
	Classname   string `json:"classname"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Add other fields as needed
}

var FurniData map[string]FurniItem

func LoadFurniData() error {
	// Assuming the executable is in the 'extension' directory
	jsonPath := filepath.Join("furnidata", "furnidata.json")

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}

	var items []FurniItem
	err = json.Unmarshal(data, &items)
	if err != nil {
		return err
	}

	FurniData = make(map[string]FurniItem)
	for _, item := range items {
		FurniData[item.Classname] = item
	}

	return nil
}

func GetIconPath(classname string) string {
	return filepath.Join("furnidata", "icons", classname+"_icon.png")
}
