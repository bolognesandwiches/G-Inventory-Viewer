package inventory

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/out"
)

type Manager struct {
	items       map[int]inventory.Item
	mutex       sync.Mutex
	isScanning  bool
	updateGUIFn func(string)
	ext         *g.Ext
}

func NewManager() *Manager {
	return &Manager{
		items: make(map[int]inventory.Item),
	}
}

func (m *Manager) SetUpdateCallback(fn func(string)) {
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

	m.updateGUIFn(fmt.Sprintf("Received %d items", len(inv.Items)))
}

func (m *Manager) ScanInventory(ext *g.Ext) {
	m.mutex.Lock()
	m.isScanning = true
	m.mutex.Unlock()

	if ext != nil {
		m.ext = ext
	}

	if m.ext == nil {
		m.updateGUIFn("Error: GoEarth extension not initialized")
		return
	}

	m.ext.Send(out.GETSTRIP, []byte("update"))

	go func() {
		time.Sleep(10 * time.Second)
		m.mutex.Lock()
		m.isScanning = false
		m.mutex.Unlock()
		m.displayInventory()
	}()
}

func (m *Manager) displayInventory() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var output strings.Builder
	output.WriteString("Current Inventory:\n------------------\n")

	itemCounts := make(map[string]int)
	for _, item := range m.items {
		key := m.getItemKey(item)
		itemCounts[key]++
	}

	var keys []string
	for key := range itemCounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		output.WriteString(fmt.Sprintf("%s: %d\n", key, itemCounts[key]))
	}

	output.WriteString("------------------\n")
	output.WriteString(fmt.Sprintf("Total unique items: %d\n", len(itemCounts)))
	output.WriteString(fmt.Sprintf("Total items: %d\n", len(m.items)))

	m.updateGUIFn(output.String())
}

func (m *Manager) getItemKey(item inventory.Item) string {
	key := fmt.Sprintf("%s (%s)", item.Class, item.Type)
	if item.Type == "I" {
		key += fmt.Sprintf(" Props:%s", item.Props)
	} else if item.Type == "S" {
		key += fmt.Sprintf(" Size:%dx%d Colors:%s", item.DimX, item.DimY, item.Colors)
	}
	return key
}
