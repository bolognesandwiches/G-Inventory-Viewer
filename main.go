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
	Title:       "g-itemViewer",
	Description: "View all items in your hand",
	Version:     "0.1.0",
	Author:      "madlad",
})

func main() {
	runtime.LockOSThread()

	inventoryManager := inventory.NewManager(ext)
	roomManager := room.NewManager(ext)
	uiManager := ui.NewManager(inventoryManager, roomManager)

	ext.Intercept(in.STRIPINFO_2).With(inventoryManager.HandleStripInfo2)

	ext.Connected(func(args g.ConnectArgs) {
		err := furnidata.LoadFurniData(args.Host)
		if err != nil {
			log.Printf("Error loading furni data: %v", err)
		}

		err = furnidata.LoadExternalTexts(args.Host)
		if err != nil {
			log.Printf("Error loading external texts: %v", err)
		}

	})

	go ext.Run()

	uiManager.Run()
}
