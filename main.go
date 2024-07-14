package main

import (
	"runtime"
	"sync"
	"time"

	"fyne.io/fyne/v2/app"
	"github.com/bolognesandwiches/G-Inventory-Viewer/common"
	"github.com/bolognesandwiches/G-Inventory-Viewer/ui"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/out"
	"xabbo.b7c.io/goearth/shockwave/profile"
	"xabbo.b7c.io/goearth/shockwave/room"
)

var ext = g.NewExt(g.ExtInfo{
	Title:       "G-itemViewer",
	Description: "Inventory and Room Viewer with Pickup and Trading utility",
	Version:     "0.3.0",
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
	pickupManager  *common.PickupManager
	profileManager = profile.NewManager(ext)
)

func init() {
	pickupManager = common.NewPickupManager(ext, inventoryMgr, nil) // We'll set the callback later in the UI manager
	SetupPacketLogging(ext)
}

func startInventoryCount() {
	lock.Lock()
	defer lock.Unlock()

	// Clear the maps
	for k := range isCounted {
		delete(isCounted, k)
	}
	for k := range retrievedItems {
		delete(retrievedItems, k)
	}

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

	fyneApp := app.New()

	uiManager = ui.NewManager(
		fyneApp,
		ext,
		inventoryMgr,
		roomMgr,
		pickupManager,
		startInventoryCount,
		profileManager,
	)

	go func() {
		for range time.Tick(time.Millisecond * 600) {
			tickCounter()
		}
	}()

	roomMgr.ObjectsLoaded(func(args room.ObjectsArgs) {
		pickupManager.UpdateRoomInfo(roomMgr.Objects, roomMgr.Items)
		go uiManager.UpdateRoomDisplay(roomMgr.Objects, roomMgr.Items)
	})

	roomMgr.ItemsLoaded(func(args room.ItemsArgs) {
		pickupManager.UpdateRoomInfo(roomMgr.Objects, roomMgr.Items)
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
