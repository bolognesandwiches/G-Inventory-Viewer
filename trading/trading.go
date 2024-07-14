package trading

import (
	"strconv"
	"sync"
	"time"

	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/out"
	"xabbo.b7c.io/goearth/shockwave/profile"
	"xabbo.b7c.io/goearth/shockwave/trade"
)

type Args struct {
	Offers Offers
}

type AcceptArgs struct {
	Name     string
	Accepted bool
}

type Offers struct {
	Trader trade.Offer
	Tradee trade.Offer
}

type Manager struct {
	*trade.Manager
	profileMgr        *profile.Manager
	inventoryMgr      *inventory.Manager
	ext               *g.Ext
	isInTrade         map[int]bool
	tradingItem       string
	tradingItemProps  string
	tradingQty        int
	targetQty         int
	didLoop           bool
	didBrowse         bool
	startedAt         int
	warnTradeDeclined bool
	lastTrade         trade.Offers
	lock              sync.Mutex
	isTradeOpen       bool
}

func (m *Manager) IsInTrade(itemId int) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.isInTrade[itemId]
}

func (m *Manager) IsTradeOpen() bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.isTradeOpen
}

func NewManager(ext *g.Ext, profileMgr *profile.Manager, inventoryMgr *inventory.Manager) *Manager {
	mgr := &Manager{
		Manager:      trade.NewManager(ext),
		profileMgr:   profileMgr,
		inventoryMgr: inventoryMgr,
		ext:          ext,
		isInTrade:    make(map[int]bool),
		isTradeOpen:  false,
	}

	mgr.Updated(mgr.handleTradeItems)
	mgr.Closed(mgr.handleTradeClose)
	mgr.Completed(mgr.handleTradeComplete)

	return mgr
}

func (m *Manager) Offer(itemId int) {
	m.Manager.Offer(itemId)
}

func (m *Manager) OfferItem(item inventory.Item) {
	m.Manager.OfferItem(item)
}

func (m *Manager) Accept() {
	m.Manager.Accept()
}

func (m *Manager) Unaccept() {
	m.Manager.Unaccept()
}

func (m *Manager) handleTradeItems(args trade.Args) {
	m.lock.Lock()
	defer m.lock.Unlock()
	clear(m.isInTrade)
	m.tradingQty = 0
	m.warnTradeDeclined = false
	m.isTradeOpen = true // Set this to true when trade starts

	for i := 0; i < 2; i++ {
		inv := args.Offers[i]
		if m.profileMgr.Profile.Name == inv.Name {
			for _, item := range inv.Items {
				m.isInTrade[item.ItemId] = true
				if item.Class == m.tradingItem {
					m.tradingQty++
				}
			}
		} else {
			m.warnTradeDeclined = len(inv.Items) > 0
		}
	}

	if m.tradingQty >= m.targetQty {
		m.targetQty = 0
		m.tradingItem = ""
		m.tradingItemProps = ""
	}
}

func (m *Manager) handleTradeComplete(args trade.Args) {
	m.warnTradeDeclined = false
	m.lastTrade = args.Offers
}

func (m *Manager) handleTradeClose(args trade.Args) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.tradingItem = ""
	m.tradingItemProps = ""
	m.targetQty = 0
	m.isTradeOpen = false // Set this to false when trade ends
	if m.warnTradeDeclined {
		m.warnTradeDeclined = false
		// Notify user about trade cancellation
	}
}

func (m *Manager) interceptTradeAddItem(e *g.Intercept) {
	m.lock.Lock()
	defer m.lock.Unlock()
	stripId, err := strconv.Atoi(string(e.Packet.ReadBytesAt(0, e.Packet.Length())))
	if err != nil {
		return
	}
	inv := m.inventoryMgr.Items()
	item, ok := inv[stripId]
	if !ok {
		return
	}
	m.tradingItem = item.Class
	m.tradingItemProps = item.Props // most posters share class, id in props
	m.didLoop = false
	m.didBrowse = false
	m.startedAt = item.Pos
}

func (m *Manager) loopTrader() {
	for range time.Tick(time.Millisecond * 550) {
		m.tickTrader()
	}
}

func (m *Manager) tickTrader() {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.tradingItem == "" || m.tradingQty >= m.targetQty {
		return
	}
	found := 0
	for _, item := range m.inventoryMgr.Items() {
		if _, ok := m.isInTrade[item.ItemId]; ok {
			continue
		}
		if item.Class != m.tradingItem || item.Props != m.tradingItemProps {
			continue
		}
		if found == 0 {
			m.OfferItem(item)
		}
		found++
	}
	if found <= 1 {
		if !m.didLoop {
			m.ext.Send(out.GETSTRIP, []byte("next"))
			m.didBrowse = true
		} else {
			m.tradingItem = ""
			m.tradingQty = 0
			m.targetQty = 0
		}
	} else {
		m.inventoryMgr.Update()
	}
}

func clear(m map[int]bool) {
	for k := range m {
		delete(m, k)
	}
}
