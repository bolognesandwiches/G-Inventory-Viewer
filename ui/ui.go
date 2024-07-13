package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/bolognesandwiches/G-Inventory-Viewer/common"
	"github.com/bolognesandwiches/G-Inventory-Viewer/trading"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/profile"
	"xabbo.b7c.io/goearth/shockwave/room"
	"xabbo.b7c.io/goearth/shockwave/trade"
)

const (
	AssetServerBaseURL = "https://raw.githubusercontent.com/bolognesandwiches/G-Inventory-Viewer/master/assets/"
	DiscordWebhookURL  = "https://discord.com/api/webhooks/1261268478195798107/gyEDyPWQjRLwrH3cyw1hqKG4vKvs9h26lqlB-LriQs2RAgJgFz2IoAkEtG9Zct856Xec"
)

type Manager struct {
	inventoryManager           *inventory.Manager
	roomManager                *room.Manager
	tradeManager               *trading.Manager
	profileManager             *profile.Manager
	window                     fyne.Window
	inventoryText              *widget.Entry
	summaryText                *widget.Entry
	iconContainer              *fyne.Container
	itemsEntry                 *widget.Entry
	roomText                   *widget.Entry
	roomSummaryText            *widget.Entry
	roomIconContainer          *fyne.Container
	roomItemsEntry             *widget.Entry
	tradeText                  *widget.Entry
	tradeSummaryText           *widget.Entry
	tradeIconContainer         *fyne.Container
	tradeOfferContainer        *fyne.Container
	otherOfferContainer        *fyne.Container
	tradeItemsEntry            *widget.Entry
	tradeOfferEntry            *widget.Entry
	otherOfferEntry            *widget.Entry
	app                        fyne.App
	mu                         sync.Mutex
	scanButton                 *customScanButton
	discordButton              *customScanButton
	roomActionButton           *customPickupButton
	tradeAcceptButton          *widget.Button
	tradeUnacceptButton        *widget.Button
	scanCallback               func()
	inventorySummaryForDiscord string
	pickupManager              *common.PickupManager
	updateRoomDisplayFunc      func(map[int]room.Object, map[int]room.Item)
	UpdateRoomDisplayLock      sync.Mutex
	ext                        *g.Ext
	lastTrade                  trading.Offers
	tradeNewContainer          *fyne.Container
	tradeNewEntry              *widget.Entry
	tradeSummaryLabel          *widget.Label
	yourTradeOffer             map[string]int
	theirTradeOffer            map[string]int
}

func NewManager(app fyne.App, ext *g.Ext, invManager *inventory.Manager, roomManager *room.Manager, pickupManager *common.PickupManager, scanCallback func(), profileManager *profile.Manager) *Manager {
	m := &Manager{
		app:               app,
		ext:               ext,
		inventoryManager:  invManager,
		roomManager:       roomManager,
		scanCallback:      scanCallback,
		pickupManager:     pickupManager,
		profileManager:    profileManager,
		tradeManager:      trading.NewManager(ext, profileManager, invManager),
		tradeNewContainer: container.NewGridWrap(fyne.NewSize(36, 36)),
		yourTradeOffer:    make(map[string]int),
		theirTradeOffer:   make(map[string]int),
	}

	pickupManager.SetOnItemPickedUp(m.UpdateRoomDisplayAfterPickup)

	// Register trade event handlers
	m.tradeManager.Updated(m.handleTradeUpdated)
	m.tradeManager.Accepted(m.handleTradeAccepted)
	m.tradeManager.Completed(m.handleTradeCompleted)
	m.tradeManager.Closed(m.handleTradeClosed)

	return m
}

// UpdateInventoryDisplay updates the inventory display.
func (m *Manager) UpdateInventoryDisplay(items map[int]inventory.Item) {
	m.mu.Lock()
	defer m.mu.Unlock()

	enrichedItems := make(map[string][]common.EnrichedInventoryItem)
	for _, item := range items {
		enrichedItem := common.EnrichInventoryItem(item)
		enrichedItems[enrichedItem.GroupKey] = append(enrichedItems[enrichedItem.GroupKey], enrichedItem)
	}

	// Set the UI summary text without icons
	m.summaryText.SetText(common.GetInventorySummary(items))

	// Generate and store the summary with icons for Discord
	m.inventorySummaryForDiscord = common.GetInventorySummary(items)

	m.iconContainer.Objects = nil

	createButton := func(items []common.EnrichedInventoryItem) *widget.Button {
		btn := widget.NewButton("", func() {
			var details strings.Builder
			details.WriteString(fmt.Sprintf("Name: %s\n", items[0].Name))
			details.WriteString(fmt.Sprintf("Count: %d\n", len(items)))
			totalHCValue := items[0].HCValue * float64(len(items))
			details.WriteString(fmt.Sprintf("HC Value: %.2f\n", totalHCValue))
			details.WriteString("Item IDs:\n")
			for _, item := range items {
				details.WriteString(fmt.Sprintf("%d\n", item.ItemId))
			}
			m.inventoryText.SetText(details.String())
		})

		btn.SetIcon(theme.AccountIcon())
		btn.Resize(fyne.NewSize(44, 44))

		go func() {
			iconURL := items[0].IconURL
			resp, err := http.Get(iconURL)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			iconData, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return
			}

			iconResource := fyne.NewStaticResource("icon", iconData)
			btn.SetIcon(iconResource)
			btn.Refresh()
		}()

		return btn
	}

	for _, items := range enrichedItems {
		button := createButton(items)
		m.iconContainer.Add(button)
	}

	m.iconContainer.Refresh()

	// Update the Trading tab's inventory display
	m.UpdateTradingInventoryDisplay(items)

	// Deactivate scan button and activate discord button
	go func() {
		time.Sleep(10 * time.Millisecond)
		m.SetScanButtonActive(false)
		m.UpdateDiscordButtonState(true)
	}()
}

func (m *Manager) isCurrentUser(name string) bool {
	return m.profileManager.Profile.Name == name
}

func (m *Manager) UpdateTradingInventoryDisplay(items map[int]inventory.Item) {
	m.tradeIconContainer.Objects = nil

	groupedItems := make(map[string][]inventory.Item)
	for _, item := range items {
		key := item.Class
		if item.Type == "I" {
			key = fmt.Sprintf("%s_%s", item.Class, item.Props)
		}
		groupedItems[key] = append(groupedItems[key], item)
	}

	for _, itemGroup := range groupedItems {
		btn := m.createGroupedItemButton(itemGroup)
		m.tradeIconContainer.Add(btn)
	}

	m.tradeIconContainer.Refresh()
}

// UpdateRoomDisplay updates the room display.
func (m *Manager) UpdateRoomDisplay(objects map[int]room.Object, items map[int]room.Item) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.roomSummaryText.SetText(common.GetRoomSummary(objects, items))

	// Clear and repopulate room icons
	m.roomIconContainer.Objects = nil

	// Repopulate room icons
	for _, obj := range objects {
		// Add icon for object (you'll need to implement this part)
		m.addObjectIcon(obj)
	}
	for _, item := range items {
		// Add icon for item (you'll need to implement this part)
		m.addItemIcon(item)
	}
	m.roomIconContainer.Refresh()

	// Update room item IDs
	var itemIDs strings.Builder
	for id := range objects {
		itemIDs.WriteString(fmt.Sprintf("%d\n", id))
	}
	for id := range items {
		itemIDs.WriteString(fmt.Sprintf("%d\n", id))
	}
	m.roomText.SetText(itemIDs.String())

	if m.updateRoomDisplayFunc != nil {
		m.updateRoomDisplayFunc(objects, items)
	}
}

func (m *Manager) handleTradeUpdated(args trade.Args) {
	fmt.Println("handleTradeUpdated called")                                                                 // Debug log
	fmt.Printf("Trader items: %d, Tradee items: %d\n", len(args.Offers[0].Items), len(args.Offers[1].Items)) // Debug log
	m.updateTradeOffers(trading.Offers{
		Trader: args.Offers[0],
		Tradee: args.Offers[1],
	})
}

func (m *Manager) handleTradeAccepted(args trade.AcceptArgs) {
	m.updateTradeOffers(m.lastTrade)
}

func (m *Manager) handleTradeCompleted(args trade.Args) {
	m.lastTrade = trading.Offers{
		Trader: args.Offers[0],
		Tradee: args.Offers[1],
	}
	m.updateTradeOffers(m.lastTrade)
}

func (m *Manager) handleTradeClosed(args trade.Args) {
	m.clearTradeOffers()
}

func (m *Manager) updateTradeOffers(offers trading.Offers) {
	m.tradeOfferContainer.Objects = nil
	m.otherOfferContainer.Objects = nil

	m.yourTradeOffer = make(map[string]int)
	m.theirTradeOffer = make(map[string]int)

	var myOffer, theirOffer trade.Offer

	if m.isCurrentUser(offers.Trader.Name) {
		myOffer = offers.Trader
		theirOffer = offers.Tradee
	} else {
		myOffer = offers.Tradee
		theirOffer = offers.Trader
	}

	for _, item := range myOffer.Items {
		btn := m.createItemButton(item)
		m.tradeOfferContainer.Add(btn)

		enrichedItem := common.EnrichInventoryItem(item)
		m.yourTradeOffer[enrichedItem.Name]++
	}

	for _, item := range theirOffer.Items {
		btn := m.createItemButton(item)
		m.otherOfferContainer.Add(btn)

		enrichedItem := common.EnrichInventoryItem(item)
		m.theirTradeOffer[enrichedItem.Name]++
	}

	m.tradeOfferContainer.Refresh()
	m.otherOfferContainer.Refresh()

	m.tradeOfferEntry.SetText(fmt.Sprintf("Items: %d\nAccepted: %t", len(myOffer.Items), myOffer.Accepted))
	m.otherOfferEntry.SetText(fmt.Sprintf("Items: %d\nAccepted: %t", len(theirOffer.Items), theirOffer.Accepted))

	m.updateNewTradeContainer()
}

func (m *Manager) updateNewTradeContainer() {
	var summary strings.Builder

	summary.WriteString("My offer:\n")
	for item, quantity := range m.yourTradeOffer {
		summary.WriteString(fmt.Sprintf("%s [%d]\n", item, quantity))
	}

	summary.WriteString("\nTheir offer:\n")
	for item, quantity := range m.theirTradeOffer {
		summary.WriteString(fmt.Sprintf("%s [%d]\n", item, quantity))
	}

	summaryText := summary.String()
	m.tradeNewEntry.SetText(summaryText)
	m.tradeNewEntry.Refresh()
}

func (m *Manager) clearTradeOffers() {
	m.tradeOfferContainer.Objects = nil
	m.otherOfferContainer.Objects = nil
	m.yourTradeOffer = make(map[string]int)
	m.theirTradeOffer = make(map[string]int)

	m.tradeOfferContainer.Refresh()
	m.otherOfferContainer.Refresh()

	m.tradeOfferEntry.SetText("")
	m.otherOfferEntry.SetText("")
	m.tradeNewEntry.SetText("")
}

func (m *Manager) createTitledContainer(content fyne.CanvasObject, title string) *fyne.Container {
	titleText := canvas.NewText(title, color.White)
	titleText.Alignment = fyne.TextAlignCenter
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.TextSize = 0
	titleText.TextStyle.Monospace = true
	return container.NewBorder(titleText, nil, nil, nil, content)
}

func (m *Manager) Run() {
	m.mu.Lock()
	m.app = app.New()
	customTheme := &habboTheme{}
	m.app.Settings().SetTheme(customTheme)
	icon, _ := m.loadImage("app_icon.ico")
	m.app.SetIcon(icon)
	m.window = m.app.NewWindow("G-itemViewer")
	m.mu.Unlock()

	leftIcon, _ := m.loadImage("left_icon.png")
	rightIcon, _ := m.loadImage("right_icon.png")
	leftIconImage := canvas.NewImageFromResource(leftIcon)
	rightIconImage := canvas.NewImageFromResource(rightIcon)
	leftIconImage.Resize(fyne.NewSize(100, 27))
	rightIconImage.Resize(fyne.NewSize(100, 27))
	leftIconImage.SetMinSize(fyne.NewSize(100, 27))
	rightIconImage.SetMinSize(fyne.NewSize(100, 27))

	titleText := canvas.NewText("G-itemViewer", color.White)
	titleText.Alignment = fyne.TextAlignCenter
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	header := container.NewHBox(
		leftIconImage,
		layout.NewSpacer(),
		titleText,
		layout.NewSpacer(),
		rightIconImage,
	)

	inventoryTab := m.setupInventoryTab()
	roomSummaryTab := m.setupRoomSummaryTab()
	tradingTab := m.setupTradingTab()

	tabs := NewCustomTabContainer(m, "Inventory", "Room", "Trading")
	m.scanButton = tabs.scanButton
	m.discordButton = tabs.discordButton
	tabs.Refresh()

	content := container.NewMax()

	updateContent := func() {
		switch tabs.selected {
		case 0:
			content.Objects = []fyne.CanvasObject{inventoryTab}
			tabs.scanButton.OnTapped = func() {
				if m.scanCallback != nil {
					m.scanCallback()
				}
			}
		case 1:
			content.Objects = []fyne.CanvasObject{roomSummaryTab}
			tabs.scanButton.OnTapped = func() {}
		case 2:
			content.Objects = []fyne.CanvasObject{tradingTab}
			tabs.scanButton.OnTapped = func() {}
		}
		content.Refresh()
	}

	updateContent()

	tabs.OnChanged = updateContent

	mainContainer := container.NewBorder(
		container.NewVBox(
			header,
			tabs,
		),
		nil, nil, nil,
		content,
	)

	m.window.SetContent(mainContainer)
	m.window.Resize(fyne.NewSize(275, 300))
	m.window.SetPadded(true)

	m.mu.Lock()
	m.window.Hide()
	m.mu.Unlock()

	m.window.ShowAndRun()
}

func (m *Manager) ShowWindow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.window != nil {
		m.window.Show()
	}
}

func (m *Manager) HideWindow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.window != nil {
		m.window.Hide()
	}
}

func (m *Manager) CloseWindow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.window != nil {
		m.window.Close()
		m.window = nil
	}
	if m.app != nil {
		m.app.Quit()
	}
}

func (m *Manager) SetScanButtonActive(active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.scanButton != nil {
		m.scanButton.SetActive(active)
		m.scanButton.Refresh()
	}
}

func (m *Manager) UpdateDiscordButtonState(active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.discordButton != nil {
		m.discordButton.SetActive(active)
		m.discordButton.Refresh()
	}
}

func (m *Manager) setupInventoryTab() *fyne.Container {
	m.inventoryText = widget.NewMultiLineEntry()
	m.inventoryText.Wrapping = fyne.TextWrapWord
	m.inventoryText.SetPlaceHolder("Item IDs")
	m.inventoryText.SetMinRowsVisible(10)

	m.summaryText = widget.NewMultiLineEntry()
	m.summaryText.Wrapping = fyne.TextWrapWord
	m.summaryText.SetPlaceHolder("Inventory Summary")
	m.summaryText.SetMinRowsVisible(10)

	m.iconContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	itemsScroll := container.NewScroll(container.NewPadded(container.NewPadded(container.NewPadded(m.iconContainer))))

	m.itemsEntry = widget.NewMultiLineEntry()
	m.itemsEntry.Wrapping = fyne.TextWrapWord
	m.itemsEntry.SetPlaceHolder("Items")
	m.itemsEntry.Disable()
	m.itemsEntry.SetMinRowsVisible(8)

	itemsContainer := container.NewStack(m.itemsEntry, itemsScroll)

	summaryContainer := m.createTitledContainer(m.summaryText, "Inventory Summary")
	itemsContainer = m.createTitledContainer(itemsContainer, "Items")
	idContainer := m.createTitledContainer(m.inventoryText, "Item IDs")

	return container.NewVBox(
		summaryContainer,
		itemsContainer,
		idContainer,
	)
}

func (m *Manager) showQuantityDialog(items []inventory.Item) {
	quantityEntry := widget.NewEntry()
	quantityEntry.SetPlaceHolder("Enter quantity")

	dialog.ShowCustomConfirm("Enter Quantity", "Confirm", "Cancel",
		container.NewVBox(
			widget.NewLabel(fmt.Sprintf("Enter quantity for %s (max %d):", common.GetItemName(items[0].Class, string(items[0].Type), items[0].Props), len(items))),
			quantityEntry,
		),
		func(confirmed bool) {
			if confirmed {
				quantity, err := strconv.Atoi(quantityEntry.Text)
				if err != nil || quantity <= 0 || quantity > len(items) {
					dialog.ShowError(errors.New("Please enter a valid number between 1 and "+strconv.Itoa(len(items))), m.window)
					return
				}
				m.addItemsToTrade(items, quantity)

				// Show a progress dialog
				progress := dialog.NewProgress("Adding Items", "Adding items to trade...", m.window)
				go func() {
					for i := 0; i < quantity; i++ {
						progress.SetValue(float64(i+1) / float64(quantity))
						time.Sleep(550 * time.Millisecond)
					}
					progress.Hide()
				}()
			}
		},
		m.window,
	)
}

func (m *Manager) setupRoomSummaryTab() *fyne.Container {
	m.roomText = widget.NewMultiLineEntry()
	m.roomText.Wrapping = fyne.TextWrapWord
	m.roomText.SetPlaceHolder("Room Item IDs")
	m.roomText.SetMinRowsVisible(10)

	m.roomSummaryText = widget.NewMultiLineEntry()
	m.roomSummaryText.Wrapping = fyne.TextWrapWord
	m.roomSummaryText.SetPlaceHolder("Room Summary")
	m.roomSummaryText.SetMinRowsVisible(10)

	m.roomIconContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	roomItemsScroll := container.NewScroll(container.NewPadded(container.NewPadded(container.NewPadded(m.roomIconContainer))))

	m.roomItemsEntry = widget.NewMultiLineEntry()
	m.roomItemsEntry.Wrapping = fyne.TextWrapWord
	m.roomItemsEntry.SetPlaceHolder("Room Items")
	m.roomItemsEntry.Disable()
	m.roomItemsEntry.SetMinRowsVisible(8)

	roomItemsContainer := container.NewStack(m.roomItemsEntry, roomItemsScroll)

	var selectedItemIds []int

	updateRoomDisplay := func(objects map[int]room.Object, items map[int]room.Item) {
		m.roomIconContainer.Objects = nil
		enrichedObjects := make(map[string][]common.EnrichedRoomObject)
		enrichedItems := make(map[string][]common.EnrichedRoomItem)

		for _, obj := range objects {
			enrichedObj := common.EnrichRoomObject(obj)
			enrichedObjects[enrichedObj.Class] = append(enrichedObjects[enrichedObj.Class], enrichedObj)
		}

		for _, item := range items {
			enrichedItem := common.EnrichRoomItem(item)
			enrichedItems[enrichedItem.Class] = append(enrichedItems[enrichedItem.Class], enrichedItem)
		}

		for _, objects := range enrichedObjects {
			btn := widget.NewButton("", func() {
				var details strings.Builder
				selectedItemIds = make([]int, 0, len(objects)) // Clear and preallocate

				details.WriteString(fmt.Sprintf("Name: %s\n", objects[0].Name))
				details.WriteString(fmt.Sprintf("Count: %d\n", len(objects)))
				totalHCValue := objects[0].HCValue * float64(len(objects))
				details.WriteString(fmt.Sprintf("HC Value: %.2f\n", totalHCValue))
				details.WriteString("Item Details:\n")
				for _, obj := range objects {
					details.WriteString(fmt.Sprintf("%d (W:%d, H:%d, X:%d, Y:%d, Dir:%d)\n",
						obj.Id, obj.Width, obj.Height, obj.X, obj.Y, obj.Direction))
					selectedItemIds = append(selectedItemIds, obj.Id)
				}
				m.roomText.SetText(details.String())
				m.roomActionButton.SetActive(false) // Enable the pickup button when items are selected
			})

			btn.SetIcon(theme.AccountIcon())
			btn.Resize(fyne.NewSize(44, 44))

			go func(iconURL string) {
				resp, err := http.Get(iconURL)
				if err != nil {
					return
				}
				defer resp.Body.Close()

				iconData, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					return
				}

				iconResource := fyne.NewStaticResource("icon", iconData)
				btn.SetIcon(iconResource)
				btn.Refresh()
			}(objects[0].IconURL)

			m.roomIconContainer.Add(btn)
		}

		for _, items := range enrichedItems {
			btn := widget.NewButton("", func() {
				var details strings.Builder
				selectedItemIds = make([]int, 0, len(items)) // Clear and preallocate

				details.WriteString(fmt.Sprintf("Name: %s\n", items[0].Name))
				details.WriteString(fmt.Sprintf("Count: %d\n", len(items)))
				totalHCValue := items[0].HCValue * float64(len(items))
				details.WriteString(fmt.Sprintf("HC Value: %.2f\n", totalHCValue))
				details.WriteString("Item Details:\n")
				for _, item := range items {
					details.WriteString(fmt.Sprintf("%d (Location: %s)\n", item.Id, item.Location))
					selectedItemIds = append(selectedItemIds, item.Id)
				}
				m.roomText.SetText(details.String())
				m.roomActionButton.SetActive(false) // Enable the pickup button when items are selected
			})

			btn.SetIcon(theme.AccountIcon())
			btn.Resize(fyne.NewSize(44, 44))

			go func(iconURL string) {
				resp, err := http.Get(iconURL)
				if err != nil {
					return
				}
				defer resp.Body.Close()

				iconData, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					return
				}

				iconResource := fyne.NewStaticResource("icon", iconData)
				btn.SetIcon(iconResource)
				btn.Refresh()
			}(items[0].IconURL)

			m.roomIconContainer.Add(btn)
		}

		m.roomIconContainer.Refresh()
	}

	roomSummaryContainer := m.createTitledContainer(m.roomSummaryText, "Room Summary")
	roomItemsContainer = m.createTitledContainer(roomItemsContainer, "Room Items")
	roomIdContainer := m.createTitledContainer(m.roomText, "Room Item IDs")

	m.roomActionButton = newCustomPickupButton(loadRoomActionIcon(), func() {
		if len(selectedItemIds) > 0 {
			m.roomActionButton.SetActive(true)
			go func() {
				m.pickupManager.PickupItems(selectedItemIds, func() {
					m.roomActionButton.SetActive(false)
					m.UpdateRoomDisplayAfterPickup() // Update one last time after all items are picked up
				})
			}()
		}
	})

	actionButtonContainer := container.NewHBox(layout.NewSpacer(), m.roomActionButton, layout.NewSpacer())

	// Store the updateRoomDisplay function in the Manager for later use
	m.updateRoomDisplayFunc = updateRoomDisplay

	return container.NewVBox(
		roomSummaryContainer,
		roomItemsContainer,
		roomIdContainer,
		actionButtonContainer,
	)
}

func (m *Manager) UpdateRoomDisplayAfterPickup() {
	m.UpdateRoomDisplayLock.Lock()
	defer m.UpdateRoomDisplayLock.Unlock()

	m.UpdateRoomDisplay(m.roomManager.Objects, m.roomManager.Items)
}

func (m *Manager) setupTradingTab() *fyne.Container {
	m.tradeText = widget.NewMultiLineEntry()
	m.tradeText.Wrapping = fyne.TextWrapWord
	m.tradeText.SetPlaceHolder("Trade Item IDs")
	m.tradeText.SetMinRowsVisible(10)

	m.tradeSummaryText = widget.NewMultiLineEntry()
	m.tradeSummaryText.Wrapping = fyne.TextWrapWord
	m.tradeSummaryText.SetPlaceHolder("Trade Summary")
	m.tradeSummaryText.SetMinRowsVisible(10)

	// Set up the trade summary container
	m.tradeIconContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	for _, item := range m.inventoryManager.Items() {
		btn := m.createItemButton(item)
		m.tradeIconContainer.Add(btn)
	}

	m.tradeNewEntry = widget.NewMultiLineEntry()
	m.tradeNewEntry.Wrapping = fyne.TextWrapWord
	m.tradeNewEntry.SetPlaceHolder("Trade Summary")
	m.tradeNewEntry.SetMinRowsVisible(8)

	tradeNewContainer := container.NewVBox(m.tradeNewEntry)

	m.tradeIconContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	tradeItemsScroll := container.NewScroll(container.NewPadded(container.NewPadded(container.NewPadded(m.tradeIconContainer))))

	m.tradeItemsEntry = widget.NewMultiLineEntry()
	m.tradeItemsEntry.Wrapping = fyne.TextWrapWord
	m.tradeItemsEntry.SetPlaceHolder("Your Inventory")
	m.tradeItemsEntry.Disable()
	m.tradeItemsEntry.SetMinRowsVisible(8)

	tradeItemsContainer := container.NewStack(m.tradeItemsEntry, tradeItemsScroll)

	m.tradeOfferContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	m.otherOfferContainer = container.NewGridWrap(fyne.NewSize(36, 36))

	tradeOfferScroll := container.NewScroll(container.NewPadded(container.NewPadded(container.NewPadded(m.tradeOfferContainer))))
	otherOfferScroll := container.NewScroll(container.NewPadded(container.NewPadded(container.NewPadded(m.otherOfferContainer))))

	m.tradeOfferEntry = widget.NewMultiLineEntry()
	m.tradeOfferEntry.Wrapping = fyne.TextWrapWord
	m.tradeOfferEntry.SetPlaceHolder("Your Offer")
	m.tradeOfferEntry.Disable()
	m.tradeOfferEntry.SetMinRowsVisible(8)

	m.otherOfferEntry = widget.NewMultiLineEntry()
	m.otherOfferEntry.Wrapping = fyne.TextWrapWord
	m.otherOfferEntry.SetPlaceHolder("Their Offer")
	m.otherOfferEntry.Disable()
	m.otherOfferEntry.SetMinRowsVisible(8)

	tradeOfferContainer := container.NewStack(m.tradeOfferEntry, tradeOfferScroll)
	otherOfferContainer := container.NewStack(m.otherOfferEntry, otherOfferScroll)

	m.tradeAcceptButton = widget.NewButton("Accept Trade", func() {
		m.tradeManager.Accept()
	})

	m.tradeUnacceptButton = widget.NewButton("Unaccept Trade", func() {
		m.tradeManager.Unaccept()
	})

	actionButtonContainer := container.NewHBox(
		layout.NewSpacer(),
		m.tradeAcceptButton,
		m.tradeUnacceptButton,
		layout.NewSpacer(),
	)

	return container.NewVBox(
		m.createTitledContainer(tradeNewContainer, "Trade Summary"),
		m.createTitledContainer(tradeItemsContainer, "Your Inventory"),
		m.createTitledContainer(tradeOfferContainer, "My Offer"),
		m.createTitledContainer(otherOfferContainer, "Their Offer"),
		actionButtonContainer,
	)
}

func (m *Manager) addItemsToTrade(items []inventory.Item, quantity int) {
	go func() {
		for i := 0; i < quantity && i < len(items); i++ {
			m.tradeManager.Offer(items[i].ItemId)
			time.Sleep(550 * time.Millisecond)
		}
	}()
}

func (m *Manager) createGroupedItemButton(items []inventory.Item) *widget.Button {
	if len(items) == 0 {
		return nil
	}

	representative := items[0]
	btn := widget.NewButton("", func() {
		m.showQuantityDialog(items)
	})

	btn.SetIcon(theme.AccountIcon())
	btn.Resize(fyne.NewSize(44, 44))

	go func() {
		iconURL := common.GetIconURL(representative.Class, string(representative.Type), representative.Props)
		resp, err := http.Get(iconURL)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		iconData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}

		iconResource := fyne.NewStaticResource("icon", iconData)
		btn.SetIcon(iconResource)
		btn.Refresh()
	}()

	return btn
}

func (m *Manager) createItemButton(item inventory.Item) *widget.Button {
	btn := widget.NewButton("", func() {
		m.showQuantityDialog([]inventory.Item{item})
	})

	btn.SetIcon(theme.AccountIcon())
	btn.Resize(fyne.NewSize(44, 44))

	go func() {
		iconURL := common.GetIconURL(item.Class, string(item.Type), item.Props)
		resp, err := http.Get(iconURL)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		iconData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}

		iconResource := fyne.NewStaticResource("icon", iconData)
		btn.SetIcon(iconResource)
		btn.Refresh()
	}()

	return btn
}

// You'll need to implement these methods
func (m *Manager) addObjectIcon(obj room.Object) {
	// Implementation here
}

func (m *Manager) addItemIcon(item room.Item) {
	// Implementation here
}

func (m *Manager) loadImage(filename string) (fyne.Resource, error) {
	url := AssetServerBaseURL + filename
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image %s: %v", filename, err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data for %s: %v", filename, err)
	}

	return fyne.NewStaticResource(filename, data), nil
}

type customScanButton struct {
	widget.Button
	icon            *canvas.Image
	label           *widget.Label
	active          bool
	isDiscordButton bool
}

func newCustomScanButton(icon fyne.Resource, tapped func()) *customScanButton {
	button := &customScanButton{}
	button.ExtendBaseWidget(button)
	button.icon = canvas.NewImageFromResource(icon)
	button.icon.FillMode = canvas.ImageFillOriginal
	button.label = widget.NewLabel("")
	button.label.Alignment = fyne.TextAlignLeading
	button.label.TextStyle = fyne.TextStyle{Bold: true}
	button.OnTapped = tapped
	button.Importance = widget.LowImportance
	button.active = false
	return button
}

func (b *customScanButton) SetActive(active bool) {
	b.active = active
	if !b.isDiscordButton {
		if active {
			b.icon.Resource = loadScanIconActive()
			b.label.SetText("Searching...")
		} else {
			b.icon.Resource = loadScanIconInactive()
			b.label.SetText("Search")
		}
	} else {
		if active {
			b.label.SetText("to Discord")
		} else {
			b.label.SetText("Discord")
		}
	}
	b.Refresh()
}

type customPickupButton struct {
	customScanButton
}

func newCustomPickupButton(icon fyne.Resource, tapped func()) *customPickupButton {
	button := &customPickupButton{}
	button.ExtendBaseWidget(button)
	button.icon = canvas.NewImageFromResource(icon)
	button.icon.FillMode = canvas.ImageFillOriginal
	button.label = widget.NewLabel("")
	button.label.Alignment = fyne.TextAlignLeading
	button.label.TextStyle = fyne.TextStyle{Bold: true}
	button.OnTapped = tapped
	button.Importance = widget.LowImportance
	button.active = false
	button.SetActive(false) // Set initial state
	return button
}

func (b *customPickupButton) SetActive(active bool) {
	b.active = active
	if active {
		b.icon.Resource = loadScanIconActive()
		b.label.SetText("Picking up...")
	} else {
		b.icon.Resource = loadScanIconInactive()
		b.label.SetText("Pick up")
	}
	b.Refresh()
}

func (b *customScanButton) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(color.NRGBA{R: 212, G: 221, B: 225, A: 255})
	background.StrokeColor = color.Black
	background.StrokeWidth = 1.35
	background.CornerRadius = 5

	return &customButtonRenderer{
		button:     b,
		background: background,
	}
}

type customButtonRenderer struct {
	button     *customScanButton
	background *canvas.Rectangle
}

func (r *customButtonRenderer) Destroy() {}

func (r *customButtonRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	iconSize := fyne.NewSize(16, 16)
	r.button.icon.Resize(iconSize)
	r.button.icon.Move(fyne.NewPos(8, (size.Height-iconSize.Height)/2))

	labelSize := r.button.label.MinSize()
	r.button.label.Resize(labelSize)
	r.button.label.Move(fyne.NewPos(20, (size.Height-labelSize.Height)/2))
}

func (r *customButtonRenderer) MinSize() fyne.Size {
	return fyne.NewSize(100, 22)
}

func (r *customButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.button.icon, r.button.label}
}

func (r *customButtonRenderer) Refresh() {
	if r.button.active {
		r.background.FillColor = color.NRGBA{R: 212, G: 221, B: 225, A: 255}
	} else {
		r.background.FillColor = color.NRGBA{R: 136, G: 173, B: 189, A: 255}
	}
	r.background.Refresh()
	r.button.icon.Refresh()
	r.button.label.Refresh()
}

type CustomTabContainer struct {
	widget.BaseWidget
	tabs          []*customTab
	selected      int
	content       fyne.CanvasObject
	OnChanged     func()
	scanButton    *customScanButton
	discordButton *customScanButton
	manager       *Manager
}

func NewCustomTabContainer(manager *Manager, items ...string) *CustomTabContainer {
	c := &CustomTabContainer{
		selected: 0,
		manager:  manager,
	}
	c.ExtendBaseWidget(c)

	for i, item := range items {
		index := i
		tab := newCustomTab(item, func() {
			if c.selected != index {
				c.selected = index
				c.Refresh()
				if c.OnChanged != nil {
					c.OnChanged()
				}
			}
		})
		c.tabs = append(c.tabs, tab)
	}

	if len(c.tabs) > 0 {
		c.tabs[0].selected = true
	}

	c.scanButton = newCustomScanButton(loadScanIconInactive(), func() {})
	c.scanButton.SetActive(false)

	c.discordButton = newCustomScanButton(loadDiscordIcon(), func() {
		// Handle Discord button click
		webhookURL := DiscordWebhookURL

		// Use the pre-formatted inventory summary for Discord
		message := c.manager.inventorySummaryForDiscord
		lines := strings.Split(message, "\n")
		var embeds []common.Embed

		// Create a single embed with all lines as fields
		embed := common.Embed{
			Title:       lines[0],                       // Assuming the first line is the title
			Description: strings.Join(lines[1:4], "\n"), // Assuming lines 1 to 3 are the description
			Color:       0x00ff00,                       // Example color
		}

		for _, line := range lines[4:] { // Skip header lines
			if strings.TrimSpace(line) == "" {
				continue
			}

			parts := strings.SplitN(line, " ", 2)
			if len(parts) < 2 {
				continue
			}

			name := parts[0]
			value := parts[1]

			field := common.Field{
				Name:   name,
				Value:  value,
				Inline: true,
			}
			embed.Fields = append(embed.Fields, field)
		}

		embeds = append(embeds, embed)

		payload := common.WebhookPayload{
			Embeds: embeds,
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			fmt.Println("Error marshaling payload:", err)
			return
		}

		resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payloadBytes))
		if err != nil {
			fmt.Println("Error sending to Discord:", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != 204 {
			fmt.Println("Error sending to Discord: failed to send message to Discord, status code:", resp.StatusCode)
			return
		}

		fmt.Println("Successfully sent to Discord")
	})
	c.discordButton.label.SetText("Discord")
	c.discordButton.isDiscordButton = true
	c.discordButton.SetActive(false)

	c.Refresh()

	return c
}

func (c *CustomTabContainer) CreateRenderer() fyne.WidgetRenderer {
	return &customTabContainerRenderer{
		container: c,
	}
}

type customTabContainerRenderer struct {
	container *CustomTabContainer
}

func (r *customTabContainerRenderer) MinSize() fyne.Size {
	width := float32(0)
	height := float32(0)
	for _, tab := range r.container.tabs {
		size := tab.MinSize()
		width += size.Width
		if size.Height > height {
			height = size.Height
		}
	}
	scanButtonSize := r.container.scanButton.MinSize()
	width = fyne.Max(width, scanButtonSize.Width*3+10) // Account for all three buttons
	height += scanButtonSize.Height - 5
	return fyne.NewSize(width, height)
}

func (r *customTabContainerRenderer) Layout(size fyne.Size) {
	tabHeight := size.Height - 5 // Keep the original tab height
	tabWidth := size.Width / float32(len(r.container.tabs))

	for i, tab := range r.container.tabs {
		tab.Resize(fyne.NewSize(tabWidth, tabHeight))
		tab.Move(fyne.NewPos(float32(i)*tabWidth, 0))
	}

	buttonSize := fyne.NewSize(100, 22)

	// Position the 'Search' button
	r.container.scanButton.Resize(buttonSize)
	r.container.scanButton.Move(fyne.NewPos(
		0,
		tabHeight-5,
	))

	// Position the 'Discord' button
	r.container.discordButton.Resize(buttonSize)
	r.container.discordButton.Move(fyne.NewPos(
		buttonSize.Width-2,
		tabHeight-5,
	))
}

func (r *customTabContainerRenderer) Refresh() {
	for i, tab := range r.container.tabs {
		tab.selected = (i == r.container.selected)
		tab.Refresh()
	}
}

func (r *customTabContainerRenderer) Objects() []fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, len(r.container.tabs)+2)
	for i, tab := range r.container.tabs {
		objects[i] = tab
	}
	objects[len(objects)-2] = r.container.scanButton
	objects[len(objects)-1] = r.container.discordButton
	return objects
}

func (r *customTabContainerRenderer) Destroy() {}

type customTab struct {
	widget.BaseWidget
	text     string
	selected bool
	onTapped func()
}

func newCustomTab(text string, onTapped func()) *customTab {
	tab := &customTab{
		text:     text,
		onTapped: onTapped,
	}
	tab.ExtendBaseWidget(tab)
	return tab
}

func (t *customTab) CreateRenderer() fyne.WidgetRenderer {
	text := canvas.NewText(t.text, color.Black)
	text.Alignment = fyne.TextAlignTrailing
	text.TextStyle = fyne.TextStyle{Bold: true}
	text.TextSize = 11

	background := canvas.NewRectangle(color.Transparent)
	outline := canvas.NewRectangle(color.Black)

	return &customTabRenderer{
		tab:        t,
		text:       text,
		background: background,
		outline:    outline,
	}
}

type customTabRenderer struct {
	tab        *customTab
	text       *canvas.Text
	background *canvas.Rectangle
	outline    *canvas.Rectangle
}

func (r *customTabRenderer) MinSize() fyne.Size {
	return fyne.NewSize(100, 40)
}

func (r *customTabRenderer) Layout(size fyne.Size) {
	r.outline.Resize(size)
	r.background.Resize(size.Subtract(fyne.NewSize(2, 2)))
	r.background.Move(fyne.NewPos(1, 1))

	textSize := r.text.MinSize()
	r.text.Resize(textSize)
	r.text.Move(fyne.NewPos(size.Width-textSize.Width-5, 5))
}

func (r *customTabRenderer) Refresh() {
	if r.tab.selected {
		r.background.FillColor = color.NRGBA{R: 212, G: 221, B: 225, A: 255}
	} else {
		r.background.FillColor = color.NRGBA{R: 136, G: 173, B: 189, A: 255}
	}
	r.text.Color = color.Black
	r.outline.StrokeColor = color.Black
	r.outline.StrokeWidth = 5
	r.outline.FillColor = color.Transparent

	r.background.CornerRadius = 5
	r.outline.CornerRadius = 5

	r.outline.Refresh()
	r.background.Refresh()
	r.text.Refresh()
}

func (r *customTabRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.outline, r.background, r.text}
}

func (r *customTabRenderer) Destroy() {}

func (t *customTab) Tapped(*fyne.PointEvent) {
	t.onTapped()
}

func loadScanIconActive() fyne.Resource {
	res, err := (&Manager{}).loadImage("scan_icon_active.png")
	if err != nil {
		return theme.SearchIcon()
	}
	return res
}

func loadScanIconInactive() fyne.Resource {
	res, err := (&Manager{}).loadImage("scan_icon_inactive.png")
	if err != nil {
		return theme.SearchIcon()
	}
	return res
}

func loadDiscordIcon() fyne.Resource {
	res, err := (&Manager{}).loadImage("discord.png")
	if err != nil {
		return theme.InfoIcon() // Fallback icon
	}
	return res
}

func loadRoomActionIcon() fyne.Resource {
	res, err := (&Manager{}).loadImage("room_action_icon.png")
	if err != nil {
		return theme.InfoIcon() // Fallback icon
	}
	return res
}

type habboTheme struct{}

func (m *habboTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 103, G: 148, B: 167, A: 255}
	case theme.ColorNameForeground:
		return color.Black
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 80, G: 120, B: 140, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 192, G: 192, B: 192, A: 255}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 212, G: 221, B: 225, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xFF}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0xB0, G: 0xB0, B: 0xB0, A: 0xFF}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 218, G: 218, B: 218, A: 255}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 99, G: 192, B: 127, A: 255}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 103, G: 148, B: 167, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 99, G: 192, B: 127, A: 255}
	case theme.ColorNameFocus:
		return color.White
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (m *habboTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *habboTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
		return resourceVolterGoldfishBoldTtf
	}
	return resourceVolterGoldfishTtf
}

func (m *habboTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 9
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameInlineIcon:
		return 30
	case theme.SizeNameScrollBar:
		return 1
	case theme.SizeNameScrollBarSmall:
		return 1
	default:
		return theme.DefaultTheme().Size(name)
	}
}

var resourceVolterGoldfishTtf = loadFontResource("Volter_Goldfish.ttf")
var resourceVolterGoldfishBoldTtf = loadFontResource("Volter_Goldfish_bold.ttf")

func loadFontResource(filename string) fyne.Resource {
	res, err := (&Manager{}).loadImage(filename)
	if err != nil {
		return theme.DefaultTextFont()
	}
	return res
}
