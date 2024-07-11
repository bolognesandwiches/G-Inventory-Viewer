package main

import (
	"log"
	"runtime"
	"sync"

	"github.com/bolognesandwiches/G-Inventory-Viewer/ui"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/room"
)

var ext = g.NewExt(g.ExtInfo{
	Title:       "G-itemViewer",
	Description: "View all items in your room and hand",
	Version:     "0.1.1",
	Author:      "madlad",
})

type ExtensionManager struct {
	inventoryManager *inventory.Manager
	roomManager      *room.Manager
	uiManager        *ui.Manager
	assetManager     *AssetManager
}

func NewExtensionManager() *ExtensionManager {
	inventoryManager := inventory.NewManager(ext)
	roomManager := room.NewManager(ext)
	uiManager := ui.NewManager(inventoryManager, roomManager)
	assetManager := NewAssetManager()

	return &ExtensionManager{
		inventoryManager: inventoryManager,
		roomManager:      roomManager,
		uiManager:        uiManager,
		assetManager:     assetManager,
	}
}

func (em *ExtensionManager) Initialize() {
	em.inventoryManager.Updated(func() {
		items := em.inventoryManager.Items()
		go em.uiManager.UpdateInventoryDisplay(items)
	})

	em.roomManager.ObjectsLoaded(func(args room.ObjectsArgs) {
		go em.uiManager.UpdateRoomDisplay(em.roomManager.Objects, em.roomManager.Items)
	})

	em.roomManager.ItemsLoaded(func(args room.ItemsArgs) {
		go em.uiManager.UpdateRoomDisplay(em.roomManager.Objects, em.roomManager.Items)
	})
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

	for err := range errChan {
		log.Printf("Asset loading error: %v", err)
	}

	log.Println("Asset loading complete")
}

func (am *AssetManager) loadFurniData(host string) error {
	err := LoadFurniData(host)
	if err != nil {
		return err
	}
	am.mu.Lock()
	am.furniDataLoaded = true
	am.mu.Unlock()
	log.Println("Furni data loaded successfully")
	return nil
}

func (am *AssetManager) loadExternalTexts(host string) error {
	err := LoadExternalTexts(host)
	if err != nil {
		return err
	}
	am.mu.Lock()
	am.externalTextsLoaded = true
	am.mu.Unlock()
	log.Println("External texts loaded successfully")
	return nil
}

func (am *AssetManager) loadIcons() error {
	err := LoadAPIItems()
	if err != nil {
		return err
	}
	am.mu.Lock()
	am.iconsLoaded = true
	am.mu.Unlock()
	log.Println("Icons loaded successfully")
	return nil
}

func (am *AssetManager) AreAssetsLoaded() bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.furniDataLoaded && am.externalTextsLoaded && am.iconsLoaded
}

func main() {
	runtime.LockOSThread()

	log.Println("Starting G-itemViewer")

	em := NewExtensionManager()
	em.Initialize()

	ext.Connected(func(args g.ConnectArgs) {
		log.Println("Connected to server:", args.Host)
		go em.assetManager.LoadAssets(args.Host)
		em.inventoryManager.Update() // Request inventory update when connected
	})

	ext.Initialized(func(args g.InitArgs) {
		log.Printf("Extension initialized (connected=%t)", args.Connected)
	})

	ext.Activated(func() {
		log.Println("Extension activated")
		em.uiManager.ShowWindow()
	})

	ext.Disconnected(func() {
		log.Println("Disconnected from server")
		em.uiManager.CloseWindow()
	})

	go func() {
		if err := ext.RunE(); err != nil {
			log.Printf("Error running extension: %v", err)
		}
	}()

	em.uiManager.Run()
}
