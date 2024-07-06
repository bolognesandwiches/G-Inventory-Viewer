package inventory

import (
	"fmt"
	"log"
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
	updateGUIFn func([]EnrichedItem)
	ext         *g.Ext
	isScanning  bool
	firstItemId int
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
	var inv inventory.Inventory
	e.Packet.Read(&inv)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if len(inv.Items) == 0 {
		m.isScanning = false
		log.Println("Received empty page, stopping scan")
		return
	}

	if m.firstItemId == 0 {
		m.firstItemId = inv.Items[0].ItemId
	} else if inv.Items[0].ItemId == m.firstItemId {
		m.isScanning = false
		log.Println("Looped back to first item, stopping scan")
		return
	}

	for _, item := range inv.Items {
		m.items[item.ItemId] = item
	}

	log.Printf("Received %d items", len(inv.Items))

	if m.isScanning {
		// Request next page
		go func() {
			time.Sleep(100 * time.Millisecond) // Small delay to avoid flooding
			m.ext.Send(out.GETSTRIP, []byte("next"))
		}()
	}

	m.updateGUIFn(m.getEnrichedItems())
}

func (m *Manager) StartScanning() {
	m.mutex.Lock()
	m.isScanning = true
	m.firstItemId = 0
	m.items = make(map[int]inventory.Item) // Clear existing items
	m.mutex.Unlock()

	m.ext.Send(out.GETSTRIP, []byte("update"))
}

func (m *Manager) getEnrichedItems() []EnrichedItem {
	var enrichedItems []EnrichedItem
	for _, item := range m.items {
		enrichedItem := EnrichedItem{
			Item: item,
		}
		if furniData, ok := furnidata.GetFurniItem(item.Class); ok {
			enrichedItem.FurniData = furniData
			enrichedItem.IconPath = furnidata.GetIconPath(item.Class)
		} else {
			enrichedItem.FurniData = furnidata.FurniItem{
				Name:        item.Class,
				Description: "No description available",
				Classname:   item.Class,
			}
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
	itemCounts := make(map[string]int)

	for _, item := range items {
		itemCounts[item.Class]++
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Total unique items: %d\n", len(itemCounts)))
	summary.WriteString(fmt.Sprintf("Total items: %d\n", len(items)))
	summary.WriteString("------------------\n")

	for class, count := range itemCounts {
		summary.WriteString(fmt.Sprintf("%s: %d\n", class, count))
	}

	return summary.String()
}
