package furnidata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type FurniItem struct {
	ID          int    `json:"id"`
	Classname   string `json:"classname"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Revision    int    `json:"revision"`
	// Add other fields as needed
}

type FurniData struct {
	RoomItemTypes struct {
		FurniType []FurniItem `json:"furnitype"`
	} `json:"roomitemtypes"`
}

var GlobalFurniData FurniData
var FurniMap map[string]FurniItem

func LoadFurniData() error {
	// Get the directory of the current file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("unable to get the current file path")
	}
	currentDir := filepath.Dir(filename)

	// Construct the path to furnidata.json
	jsonPath := filepath.Join(currentDir, "furnidata.json")

	fmt.Printf("Attempting to load furnidata from: %s\n", jsonPath)

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read furnidata.json: %w", err)
	}

	err = json.Unmarshal(data, &GlobalFurniData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal furnidata: %w", err)
	}

	// Create a map for easier access
	FurniMap = make(map[string]FurniItem)
	for _, item := range GlobalFurniData.RoomItemTypes.FurniType {
		FurniMap[item.Classname] = item
	}

	fmt.Printf("Loaded %d furni items\n", len(FurniMap))

	return nil
}

func GetFurniItem(classname string) (FurniItem, bool) {
	item, ok := FurniMap[classname]
	return item, ok
}

func GetIconPath(classname string) string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	currentDir := filepath.Dir(filename)
	return filepath.Join(currentDir, "icons", classname+"_icon.png")
}
