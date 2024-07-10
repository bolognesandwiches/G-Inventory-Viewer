package main

import (
	"log"
	"runtime"

	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/furnidata"
	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/inventory"
	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/room"
	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/ui"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/in"
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
}

func NewExtensionManager() *ExtensionManager {
	inventoryManager := inventory.NewManager(ext)
	roomManager := room.NewManager(ext)
	uiManager := ui.NewManager(inventoryManager, roomManager)

	return &ExtensionManager{
		inventoryManager: inventoryManager,
		roomManager:      roomManager,
		uiManager:        uiManager,
	}
}

func (em *ExtensionManager) Initialize() {
	// Register packet intercepts
	ext.Intercept(in.STRIPINFO_2).With(func(e *g.Intercept) {
		go em.inventoryManager.HandleStripInfo2(e)
	})
}

func main() {
	runtime.LockOSThread()

	em := NewExtensionManager()
	em.Initialize()

	// Load Furnidata and external texts upon connection
	ext.Connected(func(args g.ConnectArgs) {
		log.Println("Connected to server:", args.Host)
		go func() {
			if err := furnidata.LoadFurniData(args.Host); err != nil {
				log.Printf("Error loading furni data: %v", err)
			}
			if err := furnidata.LoadExternalTexts(args.Host); err != nil {
				log.Printf("Error loading external texts: %v", err)
			}
			if err := furnidata.LoadAPIItems(); err != nil {
				log.Printf("Error loading API items: %v", err)
			}
		}()
	})

	// Handle extension initialization
	ext.Initialized(func(args g.InitArgs) {
		log.Printf("Extension initialized (connected=%t)", args.Connected)
	})

	ext.Activated(func() {
		log.Println("Extension activated")
		em.uiManager.ShowWindow()
		go func() {
			em.inventoryManager.ScanInventory()
		}()
	})

	// Handle disconnection
	ext.Disconnected(func() {
		log.Println("Disconnected from server")
		em.inventoryManager.Reset()
		em.roomManager.Reset()
		em.uiManager.CloseWindow()
	})

	// Run the extension
	go ext.Run()

	// Run the UI manager
	em.uiManager.Run()
}
