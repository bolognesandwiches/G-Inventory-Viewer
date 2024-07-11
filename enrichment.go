package main

import (
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/room"
)

type EnrichedInventoryItem struct {
	inventory.Item
	Name    string
	IconURL string
	HCValue float64
}

type EnrichedRoomObject struct {
	room.Object
	Name    string
	IconURL string
	HCValue float64
}

type EnrichedRoomItem struct {
	room.Item
	Name    string
	IconURL string
	HCValue float64
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
		Object:  obj,
		Name:    GetItemName(obj.Class, "S", ""),
		IconURL: GetIconURL(obj.Class, "S", ""),
		HCValue: GetHCValue(GetItemName(obj.Class, "S", "")),
	}
}

func EnrichRoomItem(item room.Item) EnrichedRoomItem {
	return EnrichedRoomItem{
		Item:    item,
		Name:    GetItemName(item.Class, "I", item.Type),
		IconURL: GetIconURL(item.Class, "I", item.Type),
		HCValue: GetHCValue(GetItemName(item.Class, "I", item.Type)),
	}
}
