package inventory

import (
	"context"
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
	ctx              context.Context
	cancel           context.CancelFunc
}

type EnrichedItem struct {
	inventory.Item
	Name    string
	IconURL string
}

func NewManager(ext *g.Ext) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		inventoryMgr: inventory.NewManager(ext),
		ext:          ext,
		isCounted:    make(map[int]bool),
		scanningDone: make(chan bool),
		allItems:     make([]EnrichedItem, 0),
		isScanning:   false,
		ctx:          ctx,
		cancel:       cancel,
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
		defer func() {
			m.mu.Lock()
			m.isScanning = false
			m.mu.Unlock()

			if m.scanStateChanged != nil {
				m.scanStateChanged(false)
			}
		}()

		select {
		case <-time.After(5 * time.Second):
			m.ext.Send(out.GETSTRIP, []byte("update"))
		case <-m.ctx.Done():
			return
		}

		select {
		case <-m.scanningDone:
			m.displayInventory()
		case <-m.ctx.Done():
			return
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
		select {
		case <-time.After(500 * time.Millisecond):
			m.ext.Send(out.GETSTRIP, []byte("next"))
		case <-m.ctx.Done():
			return
		}
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
	totalHC := 0.0
	for _, item := range m.allItems {
		name := furnidata.GetItemName(item.Class, string(item.Type), item.Props)
		itemCounts[name]++
		totalHC += furnidata.GetHCValue(name)
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

func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cancel()
	m.ctx, m.cancel = context.WithCancel(context.Background())

	m.isScanning = false
	m.isCounted = make(map[int]bool)
	m.allItems = make([]EnrichedItem, 0)

	select {
	case <-m.scanningDone:
	default:
	}

	if m.scanStateChanged != nil {
		m.scanStateChanged(false)
	}
}
