package inventory

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/furnidata"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/out"
)

type Manager struct {
	inventoryMgr     *inventory.Manager
	updateGUIFn      func([]EnrichedItem)
	ext              *g.Ext
	isScanning       bool
	isCounted        map[int]bool
	mu               sync.Mutex
	scanningDone     chan bool
	allItems         []EnrichedItem
	scanStateChanged func(bool)
}

type EnrichedItem struct {
	inventory.Item
	Name    string
	IconURL string
}

func NewManager(ext *g.Ext) *Manager {
	return &Manager{
		inventoryMgr: inventory.NewManager(ext),
		ext:          ext,
		isCounted:    make(map[int]bool),
		scanningDone: make(chan bool),
		allItems:     make([]EnrichedItem, 0),
		isScanning:   false,
	}
}

func (m *Manager) SetUpdateCallback(fn func([]EnrichedItem)) {
	m.updateGUIFn = fn
}

func (m *Manager) SetScanStateChangedCallback(callback func(bool)) {
	m.scanStateChanged = callback
}

func (m *Manager) ScanInventory() {
	m.mu.Lock()
	if m.isScanning {
		m.mu.Unlock()
		return
	}
	m.isScanning = true
	m.isCounted = make(map[int]bool)
	m.allItems = make([]EnrichedItem, 0)
	m.mu.Unlock()

	if m.scanStateChanged != nil {
		m.scanStateChanged(true)
	}

	go func() {
		time.Sleep(5 * time.Second) // Wait a bit after connecting
		m.ext.Send(out.GETSTRIP, []byte("update"))
		<-m.scanningDone
		m.displayInventory()

		m.mu.Lock()
		m.isScanning = false
		m.mu.Unlock()

		if m.scanStateChanged != nil {
			m.scanStateChanged(false)
		}
	}()
}

func (m *Manager) HandleStripInfo2(e *g.Intercept) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered in HandleStripInfo2: %v", r)
		}
	}()

	m.mu.Lock()
	if !m.isScanning {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	var inv inventory.Inventory
	e.Packet.Read(&inv)

	if len(inv.Items) == 0 {
		m.scanningDone <- true
		return
	}

	newItemFound := false
	for _, item := range inv.Items {
		m.mu.Lock()
		if !m.isCounted[item.ItemId] {
			m.isCounted[item.ItemId] = true
			enrichedItem := m.enrichItem(item)
			m.allItems = append(m.allItems, enrichedItem)
			newItemFound = true
		}
		m.mu.Unlock()
	}

	if !newItemFound {
		m.scanningDone <- true
		return
	}

	go func() {
		time.Sleep(500 * time.Millisecond) // Small delay to avoid flooding
		m.ext.Send(out.GETSTRIP, []byte("next"))
	}()

	if m.updateGUIFn != nil {
		m.updateGUIFn(m.allItems)
	}
}

func (m *Manager) enrichItem(item inventory.Item) EnrichedItem {
	name := furnidata.GetItemName(item.Class, string(item.Type), item.Props)
	iconURL := furnidata.GetIconURL(item.Class, string(item.Type), item.Props)

	return EnrichedItem{
		Item:    item,
		Name:    name,
		IconURL: iconURL,
	}
}

func (m *Manager) displayInventory() {
	if m.updateGUIFn != nil {
		m.updateGUIFn(m.allItems)
	}
}

func (m *Manager) GetInventorySummary() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	itemCounts := make(map[string]int)
	for _, item := range m.allItems {
		name := furnidata.GetItemName(item.Class, string(item.Type), item.Props)
		itemCounts[name]++
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

func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isScanning = false
	m.isCounted = make(map[int]bool)
	m.allItems = make([]EnrichedItem, 0)
	// Clear the channel if it's not empty
	select {
	case <-m.scanningDone:
	default:
	}

	if m.scanStateChanged != nil {
		m.scanStateChanged(false)
	}
}
