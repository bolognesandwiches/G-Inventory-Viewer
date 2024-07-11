package main

import (
	"runtime"
	"sync"
	"time"

	"github.com/bolognesandwiches/G-Inventory-Viewer/common"
	"github.com/bolognesandwiches/G-Inventory-Viewer/ui"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/out"
	"xabbo.b7c.io/goearth/shockwave/room"
)

var ext = g.NewExt(g.ExtInfo{
	Title:       "G-itemViewer",
	Description: "View all items in your room and hand",
	Version:     "0.1.1",
	Author:      "madlad",
})

var (
	lock           sync.Mutex
	inventoryMgr   = inventory.NewManager(ext)
	roomMgr        = room.NewManager(ext)
	uiManager      *ui.Manager
	assetManager   = NewAssetManager()
	isCountingHand bool
	isCounted      = make(map[int]bool)
	refreshed      bool
	retrievedItems = make(map[int]inventory.Item)
)

func init() {
	uiManager = ui.NewManager(inventoryMgr, roomMgr, startInventoryCount)
}

func startInventoryCount() {
	lock.Lock()
	defer lock.Unlock()

	clear(isCounted)
	clear(retrievedItems)
	refreshed = false
	isCountingHand = true

	uiManager.SetScanButtonActive(true)

	inventoryMgr.Update()
}

func tickCounter() {
	lock.Lock()
	defer lock.Unlock()

	if !isCountingHand {
		return
	}

	if !refreshed {
		refreshed = true
		inventoryMgr.Update()
		return
	}

	isDone := len(inventoryMgr.Items()) == 0
	for _, item := range inventoryMgr.Items() {
		if isCounted[item.ItemId] {
			isDone = true
			continue
		}
		retrievedItems[item.ItemId] = item
		isCounted[item.ItemId] = true
	}

	if isDone {
		uiManager.UpdateInventoryDisplay(retrievedItems)
		isCountingHand = false
	} else {
		ext.Send(out.GETSTRIP, []byte("next"))
	}
}

func main() {
	runtime.LockOSThread()

	go func() {
		for range time.Tick(time.Millisecond * 600) {
			tickCounter()
		}
	}()

	roomMgr.ObjectsLoaded(func(args room.ObjectsArgs) {
		go uiManager.UpdateRoomDisplay(roomMgr.Objects, roomMgr.Items)
	})

	roomMgr.ItemsLoaded(func(args room.ItemsArgs) {
		go uiManager.UpdateRoomDisplay(roomMgr.Objects, roomMgr.Items)
	})

	ext.Connected(func(args g.ConnectArgs) {
		go assetManager.LoadAssets(args.Host)
	})

	ext.Initialized(func(args g.InitArgs) {
	})

	ext.Activated(func() {
		uiManager.ShowWindow()
	})

	ext.Disconnected(func() {
		uiManager.CloseWindow()
	})

	go func() {
		ext.RunE()
	}()

	uiManager.Run()
}

type AssetManager struct {
	furniDataLoaded     bool
	externalTextsLoaded bool
	iconsLoaded         bool
	mu                  sync.RWMutex
}

func NewAssetManager() *AssetManager {
	return &AssetManager{}
}

func (am *AssetManager) LoadAssets(host string) {
	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	wg.Add(3)
	go func() {
		defer wg.Done()
		if err := am.loadFurniData(host); err != nil {
			errChan <- err
		}
	}()

	go func() {
		defer wg.Done()
		if err := am.loadExternalTexts(host); err != nil {
			errChan <- err
		}
	}()

	go func() {
		defer wg.Done()
		if err := am.loadIcons(); err != nil {
			errChan <- err
		}
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for range errChan {
	}
}

func (am *AssetManager) loadFurniData(host string) error {
	err := common.LoadFurniData(host)
	if err != nil {
		return err
	}
	am.mu.Lock()
	am.furniDataLoaded = true
	am.mu.Unlock()
	return nil
}

func (am *AssetManager) loadExternalTexts(host string) error {
	err := common.LoadExternalTexts(host)
	if err != nil {
		return err
	}
	am.mu.Lock()
	am.externalTextsLoaded = true
	am.mu.Unlock()
	return nil
}

func (am *AssetManager) loadIcons() error {
	err := common.LoadAPIItems()
	if err != nil {
		return err
	}
	am.mu.Lock()
	am.iconsLoaded = true
	am.mu.Unlock()
	return nil
}

func (am *AssetManager) AreAssetsLoaded() bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.furniDataLoaded && am.externalTextsLoaded && am.iconsLoaded
}
