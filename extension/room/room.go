package room

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/furnidata"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/in"
	"xabbo.b7c.io/goearth/shockwave/room"
)

type Manager struct {
	ext         *g.Ext
	updateGUIFn func([]EnrichedItem)
	allItems    []EnrichedItem
	mu          sync.Mutex
	isScanning  bool
	roomObjects []room.Object
	roomItems   room.Items
	ctx         context.Context
	cancel      context.CancelFunc
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
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &Manager{
		ext:      ext,
		allItems: make([]EnrichedItem, 0),
		ctx:      ctx,
		cancel:   cancel,
	}
	ext.Intercept(in.ACTIVEOBJECTS).With(mgr.captureRoomObjects)
	ext.Intercept(in.ITEMS).With(mgr.captureRoomItems)
	return mgr
}

func (m *Manager) SetUpdateCallback(fn func([]EnrichedItem)) {
	m.updateGUIFn = fn
}

func (m *Manager) captureRoomObjects(e *g.Intercept) {
	var objects []room.Object
	e.Packet.Read(&objects)
	m.mu.Lock()
	m.roomObjects = objects
	m.mu.Unlock()
}

func (m *Manager) captureRoomItems(e *g.Intercept) {
	var items room.Items
	e.Packet.Read(&items)
	m.mu.Lock()
	m.roomItems = items
	m.mu.Unlock()
}

func (m *Manager) ScanRoom() {
	m.mu.Lock()
	if m.isScanning {
		m.mu.Unlock()
		return
	}
	m.isScanning = true
	m.allItems = make([]EnrichedItem, 0)
	roomObjects := m.roomObjects
	roomItems := m.roomItems
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.isScanning = false
			m.mu.Unlock()
		}()

		// Process room objects
		for _, obj := range roomObjects {
			select {
			case <-m.ctx.Done():
				return
			default:
				enrichedItem := m.enrichFloorItem(obj)
				m.mu.Lock()
				m.allItems = append(m.allItems, enrichedItem)
				m.mu.Unlock()
			}
		}

		// Process room items
		for _, item := range roomItems {
			select {
			case <-m.ctx.Done():
				return
			default:
				enrichedItem := m.enrichWallItem(item)
				m.mu.Lock()
				m.allItems = append(m.allItems, enrichedItem)
				m.mu.Unlock()
			}
		}

		// Update GUI with final results
		if m.updateGUIFn != nil {
			m.updateGUIFn(m.allItems)
		}
	}()
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
	m.mu.Lock()
	defer m.mu.Unlock()

	itemCounts := make(map[string]int)
	totalHC := 0.0

	for _, item := range m.allItems {
		itemCounts[item.Name]++
		hcValue := furnidata.GetHCValue(item.Name)
		totalHC += hcValue
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Total unique items: %d\n", len(itemCounts)))
	summary.WriteString(fmt.Sprintf("Total items: %d\n", len(m.allItems)))
	summary.WriteString(fmt.Sprintf("Total wealth: %.2f HC\n", totalHC))
	summary.WriteString("------------------\n")

	for name, count := range itemCounts {
		hcValue := furnidata.GetHCValue(name)
		summary.WriteString(fmt.Sprintf("%s: %d (%.2f HC)\n", name, count, hcValue))
	}

	return summary.String()
}

func (m *Manager) GetItemDetails() string {
	m.mu.Lock()
	defer m.mu.Unlock()

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

func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cancel()
	m.ctx, m.cancel = context.WithCancel(context.Background())

	m.isScanning = false
	m.allItems = make([]EnrichedItem, 0)
	m.roomObjects = nil
	m.roomItems = nil
}
