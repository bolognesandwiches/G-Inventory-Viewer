package main

import (
	"log"
	"runtime"
	"time"

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

func main() {
	runtime.LockOSThread()

	inventoryManager := inventory.NewManager(ext)
	roomManager := room.NewManager(ext)
	uiManager := ui.NewManager(inventoryManager, roomManager)

	// Register packet intercepts
	ext.Intercept(in.STRIPINFO_2).With(inventoryManager.HandleStripInfo2)

	// Load Furnidata and external texts upon connection
	ext.Connected(func(args g.ConnectArgs) {
		log.Println("Connected to server:", args.Host)
		if err := furnidata.LoadFurniData(args.Host); err != nil {
			log.Printf("Error loading furni data: %v", err)
		}
		if err := furnidata.LoadExternalTexts(args.Host); err != nil {
			log.Printf("Error loading external texts: %v", err)
		}
	})

	// Handle extension initialization
	ext.Initialized(func(args g.InitArgs) {
		log.Printf("Extension initialized (connected=%t)", args.Connected)
	})

	ext.Activated(func() {
		log.Println("Extension activated")
		inventoryManager.Reset()
		roomManager.Reset()

		// Re-register packet interceptions
		ext.Intercept(in.STRIPINFO_2).With(inventoryManager.HandleStripInfo2)

		uiManager.ShowWindow()
		go func() {
			time.Sleep(5 * time.Second)
			inventoryManager.ScanInventory()
		}()
	})

	// Handle disconnection
	ext.Disconnected(func() {
		log.Println("Disconnected from server")
		inventoryManager.Reset()
		roomManager.Reset()
		uiManager.CloseWindow() // Close the window instead of hiding it
	})

	// Run the extension
	go ext.Run()

	// Run the UI manager
	uiManager.Run()
}
