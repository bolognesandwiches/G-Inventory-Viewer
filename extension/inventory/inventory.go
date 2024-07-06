package inventory

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/furnidata"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/out"
)

type Manager struct {
	items       map[int]inventory.Item
	mutex       sync.Mutex
	isScanning  bool
	updateGUIFn func([]EnrichedItem)
	ext         *g.Ext
}

type EnrichedItem struct {
	inventory.Item
	FurniData furnidata.FurniItem
	IconPath  string
}

func NewManager() *Manager {
	return &Manager{
		items: make(map[int]inventory.Item),
	}
}

func (m *Manager) SetUpdateCallback(fn func([]EnrichedItem)) {
	m.updateGUIFn = fn
}

func (m *Manager) SetExt(ext *g.Ext) {
	m.ext = ext
}

func (m *Manager) HandleStripInfo2(e *g.Intercept) {
	if !m.isScanning {
		return
	}

	var inv inventory.Inventory
	e.Packet.Read(&inv)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, item := range inv.Items {
		m.items[item.ItemId] = item
	}

	m.updateGUIFn(m.getEnrichedItems())
}

func (m *Manager) ScanInventory(ext *g.Ext) {
	m.mutex.Lock()
	m.isScanning = true
	m.mutex.Unlock()

	if ext != nil {
		m.ext = ext
	}

	if m.ext == nil {
		m.updateGUIFn([]EnrichedItem{{Item: inventory.Item{Class: "Error"}, FurniData: furnidata.FurniItem{Name: "Error: GoEarth extension not initialized"}}})
		return
	}

	m.ext.Send(out.GETSTRIP, []byte("update"))

	go func() {
		time.Sleep(10 * time.Second)
		m.mutex.Lock()
		m.isScanning = false
		m.mutex.Unlock()
		m.updateGUIFn(m.getEnrichedItems())
	}()
}

func (m *Manager) getEnrichedItems() []EnrichedItem {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var enrichedItems []EnrichedItem
	for _, item := range m.items {
		enrichedItem := EnrichedItem{
			Item: item,
		}
		if furniData, ok := furnidata.FurniData[item.Class]; ok {
			enrichedItem.FurniData = furniData
			enrichedItem.IconPath = furnidata.GetIconPath(item.Class)
		} else {
			enrichedItem.FurniData = furnidata.FurniItem{Name: item.Class, Description: "No description available"}
			enrichedItem.IconPath = furnidata.GetIconPath("unknown")
		}
		enrichedItems = append(enrichedItems, enrichedItem)
	}

	sort.Slice(enrichedItems, func(i, j int) bool {
		return enrichedItems[i].FurniData.Name < enrichedItems[j].FurniData.Name
	})

	return enrichedItems
}

func (m *Manager) GetInventorySummary() string {
	items := m.getEnrichedItems()
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Total items: %d\n", len(items)))
	summary.WriteString("------------------\n")
	for _, item := range items {
		summary.WriteString(fmt.Sprintf("%s (%s): %s\n", item.FurniData.Name, item.Class, item.FurniData.Description))
	}
	return summary.String()
}
