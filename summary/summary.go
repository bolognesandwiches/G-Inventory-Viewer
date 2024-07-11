package summary

import (
	"fmt"
	"strings"

	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/room"
)

func GetInventorySummary(items map[int]inventory.Item) string {
	itemCounts := make(map[string]int)
	totalHC := 0.0

	for _, item := range items {
		name := main.GetItemName(item.Class, string(item.Type), item.Props)
		itemCounts[name]++
		totalHC += main.GetHCValue(name)
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Total unique items: %d\n", len(itemCounts)))
	summary.WriteString(fmt.Sprintf("Total items: %d\n", len(items)))
	summary.WriteString(fmt.Sprintf("Total wealth: %.2f HC\n", totalHC))
	summary.WriteString("------------------\n")

	for name, count := range itemCounts {
		hcValue := main.GetHCValue(name)
		summary.WriteString(fmt.Sprintf("%s: %d (%.2f HC)\n", name, count, hcValue))
	}

	return summary.String()
}

func GetRoomSummary(objects map[int]room.Object, items map[int]room.Item) string {
	itemCounts := make(map[string]int)
	totalHC := 0.0

	for _, obj := range objects {
		name := main.GetItemName(obj.Class, "S", "")
		itemCounts[name]++
		totalHC += main.GetHCValue(name)
	}

	for _, item := range items {
		name := main.GetItemName(item.Class, "I", item.Type)
		itemCounts[name]++
		totalHC += main.GetHCValue(name)
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Total unique items: %d\n", len(itemCounts)))
	summary.WriteString(fmt.Sprintf("Total items: %d\n", len(objects)+len(items)))
	summary.WriteString(fmt.Sprintf("Total wealth: %.2f HC\n", totalHC))
	summary.WriteString("------------------\n")

	for name, count := range itemCounts {
		hcValue := main.GetHCValue(name)
		summary.WriteString(fmt.Sprintf("%s: %d (%.2f HC)\n", name, count, hcValue))
	}

	return summary.String()
}

func GetInventoryItemDetails(item inventory.Item) string {
	name := main.GetItemName(item.Class, string(item.Type), item.Props)
	return fmt.Sprintf("Name: %s\nID: %d\nType: %s\nClass: %s\nProps: %s\n",
		name, item.ItemId, item.Type, item.Class, item.Props)
}

func GetRoomItemDetails(item room.Item) string {
	name := main.GetItemName(item.Class, "I", item.Type)
	return fmt.Sprintf("Name: %s\nID: %d\nClass: %s\nOwner: %s\nLocation: %s\nType: %s\n",
		name, item.Id, item.Class, item.Owner, item.Location, item.Type)
}

func GetRoomObjectDetails(obj room.Object) string {
	name := main.GetItemName(obj.Class, "S", "")
	return fmt.Sprintf("Name: %s\nID: %d\nClass: %s\nPosition: (%d, %d, %.2f)\nSize: %dx%d\nDirection: %d\n",
		name, obj.Id, obj.Class, obj.X, obj.Y, obj.Z, obj.Width, obj.Height, obj.Direction)
}
