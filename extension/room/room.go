package room

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/furnidata"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/in"
	"xabbo.b7c.io/goearth/shockwave/room"
)

type Manager struct {
	ext         *g.Ext
	updateGUIFn func([]EnrichedItem)
	allItems    []EnrichedItem
}

type EnrichedItem struct {
	Id        string
	Class     string
	Type      string // "S" for floor items, "I" for wall items
	Name      string
	IconURL   string
	Width     int
	Height    int
	X         int
	Y         int
	Direction int
	Location  string
}

func NewManager(ext *g.Ext) *Manager {
	return &Manager{
		ext:      ext,
		allItems: make([]EnrichedItem, 0),
	}
}

func (m *Manager) SetUpdateCallback(fn func([]EnrichedItem)) {
	m.updateGUIFn = fn
}

func (m *Manager) ScanRoom() {
	m.allItems = make([]EnrichedItem, 0)
	m.ext.Intercept(in.ACTIVEOBJECTS).With(m.handleActiveObjects)
	m.ext.Intercept(in.ITEMS).With(m.handleItems)
}

func (m *Manager) handleActiveObjects(e *g.Intercept) {
	var objects []room.Object
	e.Packet.Read(&objects)

	for _, obj := range objects {
		enrichedItem := m.enrichFloorItem(obj)
		m.allItems = append(m.allItems, enrichedItem)
	}

	if m.updateGUIFn != nil {
		m.updateGUIFn(m.allItems)
	}
}

func (m *Manager) handleItems(e *g.Intercept) {
	var items room.Items
	e.Packet.Read(&items)

	for _, item := range items {
		enrichedItem := m.enrichWallItem(item)
		m.allItems = append(m.allItems, enrichedItem)
	}

	if m.updateGUIFn != nil {
		m.updateGUIFn(m.allItems)
	}
}

func (m *Manager) enrichFloorItem(obj room.Object) EnrichedItem {
	name := furnidata.GetItemName(obj.Class, "S", "")
	if name == "" {
		name = obj.Class
	}
	iconURL := furnidata.GetIconURL(obj.Class, "S", "")

	return EnrichedItem{
		Id:        obj.Id,
		Class:     obj.Class,
		Type:      "S",
		Name:      name,
		IconURL:   iconURL,
		Width:     obj.Width,
		Height:    obj.Height,
		X:         obj.X,
		Y:         obj.Y,
		Direction: obj.Direction,
	}
}

func (m *Manager) enrichWallItem(item room.Item) EnrichedItem {
	name := furnidata.GetItemName(item.Class, "I", item.Type)
	if name == "" {
		name = item.Class
		if item.Type != "" {
			name += "_" + item.Type
		}
	}
	iconURL := furnidata.GetIconURL(item.Class, "I", item.Type)

	return EnrichedItem{
		Id:       strconv.Itoa(item.Id),
		Class:    item.Class,
		Type:     "I",
		Name:     name,
		IconURL:  iconURL,
		Location: item.Location,
	}
}

func (m *Manager) GetRoomSummary() string {
	itemCounts := make(map[string]int)

	for _, item := range m.allItems {
		itemCounts[item.Name]++
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Total unique items: %d\n", len(itemCounts)))
	summary.WriteString(fmt.Sprintf("Total items: %d\n", len(m.allItems)))
	summary.WriteString("------------------\n")

	for name, count := range itemCounts {
		summary.WriteString(fmt.Sprintf("%s: %d\n", name, count))
	}

	return summary.String()
}

func (m *Manager) GetItemDetails() string {
	var details strings.Builder

	for _, item := range m.allItems {
		details.WriteString(fmt.Sprintf("Name: %s\n", item.Name))
		details.WriteString(fmt.Sprintf("Count: %d\n", 1)) // Assuming each item is unique in the room
		details.WriteString("IDs:\n")

		if item.Type == "S" {
			details.WriteString(fmt.Sprintf("%s (W:%d, H:%d, X:%d, Y:%d, Dir:%d)\n",
				item.Id, item.Width, item.Height, item.X, item.Y, item.Direction))
		} else {
			details.WriteString(fmt.Sprintf("%s (Location: %s)\n", item.Id, item.Location))
		}

		details.WriteString("\n")
	}

	return details.String()
}
