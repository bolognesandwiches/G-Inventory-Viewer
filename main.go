package main

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/in"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/out"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var ext = g.NewExt(g.ExtInfo{
	Title:       "Inventory Viewer",
	Description: "View all items in your hand",
	Version:     "0.3.0",
	Author:      "Modified from 0xb0bba's G-Trader",
})

var inventoryItems = make(map[int]inventory.Item)
var isScanning = false
var scanComplete = make(chan bool)
var mutex = &sync.Mutex{}
var firstItemId int
var pageCount = 0

var inventoryText *widget.Entry
var window fyne.Window

func main() {
	runtime.LockOSThread()

	myApp := app.New()
	window = myApp.NewWindow("Habbo Inventory Viewer")

	inventoryText = widget.NewMultiLineEntry()
	inventoryText.SetText("Waiting for inventory scan...")

	content := container.NewVBox(
		widget.NewLabel("Inventory:"),
		inventoryText,
	)

	window.SetContent(content)

	go func() {
		ext.Intercept(in.STRIPINFO_2).With(handleStripInfo2)
		ext.Connected(func(args g.ConnectArgs) {
			updateGUI("Connected to server. Starting inventory scan...")
			go scanInventory()
		})
		ext.Run()
	}()

	window.Resize(fyne.NewSize(600, 400))
	window.Show()

	myApp.Run()
}

func handleStripInfo2(e *g.Intercept) {
	if !isScanning {
		return
	}

	var inv inventory.Inventory
	e.Packet.Read(&inv)

	mutex.Lock()
	defer mutex.Unlock()

	pageCount++
	updateGUI(fmt.Sprintf("Received STRIPINFO_2 packet. Page: %d, Items count: %d\n", pageCount, len(inv.Items)))

	if len(inv.Items) == 0 {
		isScanning = false
		scanComplete <- true
		return
	}

	if firstItemId == 0 {
		firstItemId = inv.Items[0].ItemId
	} else if inv.Items[0].ItemId == firstItemId {
		isScanning = false
		scanComplete <- true
		return
	}

	for _, item := range inv.Items {
		inventoryItems[item.ItemId] = item
	}

	// Request next page
	go func() {
		time.Sleep(time.Millisecond * 500) // Small delay to avoid flooding
		ext.Send(out.GETSTRIP, []byte("next"))
	}()
}

func scanInventory() {
	time.Sleep(5 * time.Second) // Wait a bit after connecting

	updateGUI("Starting inventory scan...")
	isScanning = true
	firstItemId = 0
	pageCount = 0
	ext.Send(out.GETSTRIP, []byte("update"))

	// Wait for scan to complete
	<-scanComplete

	displayInventory()
}

func displayInventory() {
	var output strings.Builder

	output.WriteString("Current Inventory:\n------------------\n")

	itemCounts := make(map[string]int)
	itemIDs := make(map[string][]int)
	mutex.Lock()
	for id, item := range inventoryItems {
		key := getItemKey(item)
		itemCounts[key]++
		itemIDs[key] = append(itemIDs[key], id)
	}
	mutex.Unlock()

	// Sort keys for consistent output
	var keys []string
	for key := range itemCounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		count := itemCounts[key]
		ids := itemIDs[key]
		sort.Ints(ids)

		output.WriteString(fmt.Sprintf("%s: %d\n", key, count))
		output.WriteString(fmt.Sprintf("  IDs: %v\n", ids))
	}

	output.WriteString("------------------\n")
	output.WriteString(fmt.Sprintf("Total unique items: %d\n", len(itemCounts)))
	output.WriteString(fmt.Sprintf("Total items: %d\n", len(inventoryItems)))
	output.WriteString(fmt.Sprintf("Total pages scanned: %d\n", pageCount))

	updateGUI(output.String())
}

func getItemKey(item inventory.Item) string {
	key := fmt.Sprintf("%s (%s)", item.Class, item.Type)
	if item.Type == "I" {
		key += fmt.Sprintf(" Props:%s", item.Props)
	} else if item.Type == "S" {
		key += fmt.Sprintf(" Size:%dx%d Colors:%s", item.DimX, item.DimY, item.Colors)
	}
	return key
}

func updateGUI(text string) {
	if inventoryText != nil {
		inventoryText.SetText(text)
	}
}
