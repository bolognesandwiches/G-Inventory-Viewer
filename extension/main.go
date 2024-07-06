package main

import (
	"log"
	"runtime"

	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/furnidata"
	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/inventory"
	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/ui"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/in"
)

var ext = g.NewExt(g.ExtInfo{
	Title:       "Inventory Viewer",
	Description: "View all items in your hand",
	Version:     "0.4.0",
	Author:      "Modified from 0xb0bba's G-Trader",
})

func main() {
	runtime.LockOSThread()

	err := furnidata.LoadFurniData()
	if err != nil {
		log.Fatalf("Failed to load furni data: %v", err)
	}

	inventoryManager := inventory.NewManager()
	uiManager := ui.NewManager(inventoryManager)

	go func() {
		ext.Intercept(in.STRIPINFO_2).With(func(e *g.Intercept) {
			inventoryManager.HandleStripInfo2(e)
		})
		ext.Connected(func(args g.ConnectArgs) {
			log.Println("Connected to server. Starting inventory scan...")
			go inventoryManager.ScanInventory(ext)
		})
		ext.Run()
	}()

	uiManager.Run()
}
