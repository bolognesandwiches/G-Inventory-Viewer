package common

import (
	"fmt"
	"time"

	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/out"
	"xabbo.b7c.io/goearth/shockwave/room"
)

type PickupManager struct {
	ext              *g.Ext
	inventoryManager *inventory.Manager
	roomObjects      map[int]room.Object
	roomItems        map[int]room.Item
	onItemPickedUp   func()
}

func NewPickupManager(ext *g.Ext, inventoryManager *inventory.Manager, onItemPickedUp func()) *PickupManager {
	return &PickupManager{
		ext:              ext,
		inventoryManager: inventoryManager,
		roomObjects:      make(map[int]room.Object),
		roomItems:        make(map[int]room.Item),
		onItemPickedUp:   onItemPickedUp,
	}
}

func (pm *PickupManager) UpdateRoomInfo(objects map[int]room.Object, items map[int]room.Item) {
	pm.roomObjects = objects
	pm.roomItems = items
}

func (pm *PickupManager) PickupItems(itemIds []int, onComplete func()) {
	for i, id := range itemIds {
		fmt.Printf("Attempting to pick up item %d (%.2f%% complete)\n", id, float64(i+1)/float64(len(itemIds))*100)

		var packetData string
		if _, isFloorItem := pm.roomObjects[id]; isFloorItem {
			packetData = fmt.Sprintf("new stuff %d", id)
			delete(pm.roomObjects, id)
		} else if _, isWallItem := pm.roomItems[id]; isWallItem {
			packetData = fmt.Sprintf("new item %d", id)
			delete(pm.roomItems, id)
		} else {
			fmt.Printf("Item %d not found in room\n", id)
			continue
		}

		pm.ext.Send(out.ADDSTRIPITEM, []byte(packetData))
		fmt.Printf("Sent pickup packet for item %d: %s\n", id, packetData)

		// Call the onItemPickedUp callback
		if pm.onItemPickedUp != nil {
			pm.onItemPickedUp()
		}

		time.Sleep(550 * time.Millisecond)
	}
	fmt.Println("Pickup process completed")
	if onComplete != nil {
		onComplete()
	}
}

func (pm *PickupManager) SetOnItemPickedUp(callback func()) {
	pm.onItemPickedUp = callback
}
