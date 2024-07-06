package main

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/in"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/out"
)

var ext = g.NewExt(g.ExtInfo{
	Title:       "Inventory Viewer",
	Description: "View all items in your hand",
	Version:     "0.2.5",
	Author:      "madlad",
})

var inventoryItems = make(map[int]inventory.Item)
var isScanning = false
var scanComplete = make(chan bool)
var mutex = &sync.Mutex{}
var firstItemId int
var pageCount = 0

func main() {
	ext.Intercept(in.STRIPINFO_2).With(handleStripInfo2)
	ext.Connected(func(args g.ConnectArgs) {
		log.Println("Connected to server. Starting inventory scan...")
		go scanInventory()
	})
	ext.Run()
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
	log.Printf("Received STRIPINFO_2 packet. Page: %d, Items count: %d\n", pageCount, len(inv.Items))

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

	log.Println("Starting inventory scan...")
	isScanning = true
	firstItemId = 0
	pageCount = 0
	ext.Send(out.GETSTRIP, []byte("update"))

	// Wait for scan to complete
	<-scanComplete

	displayInventory()
}

func displayInventory() {
	log.Println("\nCurrent Inventory:")
	log.Println("------------------")
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

		log.Printf("%s: %d\n", key, count)
		log.Printf("  IDs: %v\n", ids)
	}
	log.Println("------------------")
	log.Printf("Total unique items: %d\n", len(itemCounts))
	log.Printf("Total items: %d\n", len(inventoryItems))
	log.Printf("Total pages scanned: %d\n", pageCount)
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
