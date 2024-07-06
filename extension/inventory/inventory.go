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
	var inv inventory.Inventory
	e.Packet.Read(&inv)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, item := range inv.Items {
		m.items[item.ItemId] = item
	}

	log.Printf("Received %d items", len(inv.Items))

	if len(inv.Items) == 0 {
		m.isScanning = false
	} else {
		// Request next page
		go func() {
			time.Sleep(100 * time.Millisecond) // Small delay to avoid flooding
			m.ext.Send(out.GETSTRIP, []byte("next"))
		}()
	}

	m.updateGUIFn(m.getEnrichedItems())
}

func (m *Manager) ScanInventory() {
	m.mutex.Lock()
	m.isScanning = true
	m.mutex.Unlock()

	if m.ext == nil {
		log.Println("Error: GoEarth extension not initialized")
		return
	}

	log.Println("Sending GETSTRIP update...")
	m.ext.Send(out.GETSTRIP, []byte("update"))

	timeout := time.After(30 * time.Second)
	tick := time.Tick(500 * time.Millisecond)

	for {
		select {
		case <-timeout:
			log.Println("Inventory scan timed out")
			m.mutex.Lock()
			m.isScanning = false
			m.mutex.Unlock()
			return
		case <-tick:
			if !m.isScanning {
				log.Println("Inventory scan completed")
				m.updateGUIFn(m.getEnrichedItems())
				return
			}
		}
	}
}

func (m *Manager) getEnrichedItems() []EnrichedItem {
	m.mutex.Lock()
	defer m.mutex.Unlock()

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
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Total items: %d\n", len(items)))
	summary.WriteString("------------------\n")
	for _, item := range items {
		summary.WriteString(fmt.Sprintf("%s (%s): %s\n", item.FurniData.Name, item.Class, item.FurniData.Description))
	}
	return summary.String()
}
