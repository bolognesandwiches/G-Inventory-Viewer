package ui

import (
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

type UnifiedItem struct {
	Item         inventory.Item
	EnrichedItem common.EnrichedInventoryItem
	Quantity     int
	InTrade      bool
}

type UnifiedInventory struct {
	Items   map[int]UnifiedItem // Key is ItemId
	Summary InventorySummary
	mu      sync.RWMutex
}

type InventorySummaryItem struct {
	Quantity int
	HCValue  float64
}

type InventorySummary struct {
	TotalUniqueItems int
	TotalItems       int
	TotalWealth      float64
	Items            map[string]InventorySummaryItem
}

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
	scanCallback               func()
	inventorySummaryForDiscord string
	pickupManager              *common.PickupManager
	updateRoomDisplayFunc      func(map[int]room.Object, map[int]room.Item)
	UpdateRoomDisplayLock      sync.Mutex
	ext                        *g.Ext
	lastTrade                  trading.Offers
	tradeNewContainer          *fyne.Container
	tradeNewEntry              *widget.Entry
	yourTradeOffer             map[string]int
	theirTradeOffer            map[string]int
	unifiedInventory           *UnifiedInventory
	inventoryWindow            fyne.Window
	tradeManagerWindow         fyne.Window
	quantityDialog             *dialog.CustomDialog
}

func NewUnifiedInventory() *UnifiedInventory {
	return &UnifiedInventory{
		Items: make(map[int]UnifiedItem),
		Summary: InventorySummary{
			Items: make(map[string]InventorySummaryItem),
		},
	}
}

func (m *Manager) showQuantityDialogInTradeManager(items []UnifiedItem) {
	if len(items) == 0 || m.tradeManagerWindow == nil {
		return
	}

	quantityEntry := widget.NewEntry()
	representative := items[0]

	maxQuantity := 0
	for _, item := range items {
		if !item.InTrade {
			maxQuantity += item.Quantity
		}
	}

	dialogContent := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Enter quantity for %s (max %d):", representative.EnrichedItem.Name, maxQuantity)),
		quantityEntry,
	)

	dialogContentWrapper := m.createStyledContainerWithButtons(dialogContent, "")

	dialog.ShowCustomConfirm("", "Confirm", "Cancel", dialogContentWrapper,
		func(confirmed bool) {
			if confirmed {
				quantity, err := strconv.Atoi(quantityEntry.Text)
				if err != nil || quantity <= 0 || quantity > maxQuantity {
					dialog.ShowError(errors.New("Please enter a valid number between 1 and "+strconv.Itoa(maxQuantity)), m.tradeManagerWindow)
					return
				}
				m.addItemsToTrade(items, quantity)

				progress := dialog.NewProgress("Adding Items", "Adding items to trade...", m.tradeManagerWindow)
				go func() {
					for i := 0; i < quantity; i++ {
						progress.SetValue(float64(i+1) / float64(quantity))
						time.Sleep(550 * time.Millisecond)
					}
					progress.Hide()
				}()
			}
		},
		m.tradeManagerWindow,
	)
}

func (m *Manager) createTradeManagerWindow() fyne.Window {
	tradeWindow := m.app.NewWindow("Trade Manager")
	tradeWindow.SetIcon(m.window.Icon())

	m.tradeNewEntry = widget.NewMultiLineEntry()
	m.tradeNewEntry.Wrapping = fyne.TextWrapWord
	m.tradeNewEntry.SetPlaceHolder("Awaiting trade...")
	m.tradeNewEntry.SetMinRowsVisible(8)

	tradeSummaryContainer := m.createStyledMultiLineEntryContainer(m.tradeNewEntry, "Trade Summary")

	m.tradeOfferContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	m.otherOfferContainer = container.NewGridWrap(fyne.NewSize(36, 36))

	tradeOfferScroll := m.createStyledScrollContainer(m.tradeOfferContainer, "Your Offer")
	otherOfferScroll := m.createStyledScrollContainer(m.otherOfferContainer, "Their Offer")

	m.tradeAcceptButton = widget.NewButton("Accept Trade", func() {
		m.showTradeConfirmationDialog()
	})

	actionButtonContainer := container.NewHBox(
		layout.NewSpacer(),
		m.tradeAcceptButton,
		layout.NewSpacer(),
	)

	content := container.NewVBox(
		tradeSummaryContainer,
		tradeOfferScroll,
		otherOfferScroll,
		actionButtonContainer,
	)

	tradeWindow.SetContent(content)
	tradeWindow.Resize(fyne.NewSize(400, 600))

	tradeWindow.SetCloseIntercept(func() {
		tradeWindow.Hide()
	})

	return tradeWindow
}
func (m *Manager) createTradeManagerWindowContent() fyne.CanvasObject {
	m.tradeNewEntry = widget.NewMultiLineEntry()
	m.tradeNewEntry.Wrapping = fyne.TextWrapWord
	m.tradeNewEntry.SetPlaceHolder("Trade Summary")
	m.tradeNewEntry.SetMinRowsVisible(8)

	tradeSummaryContainer := m.createStyledMultiLineEntryContainer(m.tradeNewEntry, "Trade Summary")

	m.tradeOfferContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	m.otherOfferContainer = container.NewGridWrap(fyne.NewSize(36, 36))

	tradeOfferScroll := m.createStyledScrollContainer(m.tradeOfferContainer, "Your Offer")
	otherOfferScroll := m.createStyledScrollContainer(m.otherOfferContainer, "Their Offer")

	m.tradeAcceptButton = widget.NewButton("Accept Trade", func() {
		m.showTradeConfirmationDialog()
	})

	actionButtonContainer := container.NewHBox(
		layout.NewSpacer(),
		m.tradeAcceptButton,
		layout.NewSpacer(),
	)

	return container.NewVBox(
		tradeSummaryContainer,
		tradeOfferScroll,
		otherOfferScroll,
		actionButtonContainer,
	)
}

func (m *Manager) ShowTradeManagerWindow() {
	if m.tradeManagerWindow == nil {
		m.tradeManagerWindow = m.createTradeManagerWindow()
	}
	m.tradeManagerWindow.Show()
}
func (m *Manager) setupInventoryTab() *fyne.Container {
	m.inventoryText = widget.NewMultiLineEntry()
	m.inventoryText.Wrapping = fyne.TextWrapWord
	m.inventoryText.SetPlaceHolder("Open your Inventory and then click on Item icons to view more information.")
	m.inventoryText.SetMinRowsVisible(10)

	m.summaryText = widget.NewMultiLineEntry()
	m.summaryText.Wrapping = fyne.TextWrapWord
	m.summaryText.SetPlaceHolder("Click on 'Scan' to begin scanning your inventory!")
	m.summaryText.SetMinRowsVisible(10)

	openInventoryButton := widget.NewButton("Open Inventory", func() {
		m.ShowInventoryWindow()
	})

	openTradeManagerButton := widget.NewButton("Open Trade Manager", func() {
		m.ShowTradeManagerWindow()
	})

	summaryContainer := m.createStyledMultiLineEntryContainer(m.summaryText, "Inventory Summary")
	idContainer := m.createStyledMultiLineEntryContainer(m.inventoryText, "Item Details")

	return container.NewVBox(
		summaryContainer,
		idContainer,
		openInventoryButton,
		openTradeManagerButton,
	)
}

func (ui *UnifiedInventory) AddItem(item inventory.Item) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	enrichedItem := common.EnrichInventoryItem(item)
	unifiedItem, exists := ui.Items[item.ItemId]
	if !exists {
		unifiedItem = UnifiedItem{
			Item:         item,
			EnrichedItem: enrichedItem,
			Quantity:     1,
			InTrade:      false,
		}
		ui.Summary.TotalUniqueItems++
	} else {
		unifiedItem.Quantity++
	}
	ui.Items[item.ItemId] = unifiedItem

	ui.Summary.TotalItems++
	ui.Summary.TotalWealth += enrichedItem.HCValue

	summaryItem := ui.Summary.Items[enrichedItem.Name]
	summaryItem.Quantity++
	summaryItem.HCValue = enrichedItem.HCValue
	ui.Summary.Items[enrichedItem.Name] = summaryItem
}

func (ui *UnifiedInventory) RemoveItem(itemId int) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	unifiedItem, exists := ui.Items[itemId]
	if !exists {
		return
	}

	unifiedItem.Quantity--
	if unifiedItem.Quantity <= 0 {
		delete(ui.Items, itemId)
		ui.Summary.TotalUniqueItems--
	} else {
		ui.Items[itemId] = unifiedItem
	}

	ui.Summary.TotalItems--
	ui.Summary.TotalWealth -= unifiedItem.EnrichedItem.HCValue

	summaryItem := ui.Summary.Items[unifiedItem.EnrichedItem.Name]
	summaryItem.Quantity--
	if summaryItem.Quantity <= 0 {
		delete(ui.Summary.Items, unifiedItem.EnrichedItem.Name)
	} else {
		ui.Summary.Items[unifiedItem.EnrichedItem.Name] = summaryItem
	}
}

func (ui *UnifiedInventory) UpdateItemTradeStatus(itemId int, inTrade bool) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if item, exists := ui.Items[itemId]; exists {
		item.InTrade = inTrade
		ui.Items[itemId] = item
	}
}

func (ui *UnifiedInventory) GetGroupedItems() map[string][]UnifiedItem {
	ui.mu.RLock()
	defer ui.mu.RUnlock()

	grouped := make(map[string][]UnifiedItem)
	for _, item := range ui.Items {
		key := item.EnrichedItem.GroupKey
		grouped[key] = append(grouped[key], item)
	}
	return grouped
}

func (ui *UnifiedInventory) GetSummary() InventorySummary {
	ui.mu.RLock()
	defer ui.mu.RUnlock()

	return ui.Summary
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
		unifiedInventory:  NewUnifiedInventory(),
		iconContainer:     container.NewGridWrap(fyne.NewSize(36, 36)), // Ensure this is initialized
	}

	pickupManager.SetOnItemPickedUp(m.UpdateRoomDisplayAfterPickup)

	// Register trade event handlers
	m.tradeManager.Updated(m.handleTradeUpdated)
	m.tradeManager.Accepted(m.handleTradeAccepted)
	m.tradeManager.Completed(m.handleTradeCompleted)
	m.tradeManager.Closed(m.handleTradeClosed)

	return m
}

func (m *Manager) UpdateInventoryDisplay(items map[int]inventory.Item) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing items and summary
	m.unifiedInventory = NewUnifiedInventory()

	for _, item := range items {
		m.unifiedInventory.AddItem(item)
	}

	m.RefreshInventorySummaryDisplay()
	m.RefreshInventoryIcons()
	m.RefreshTradingInventoryDisplay()

	// Deactivate scan button and activate discord button
	go func() {
		time.Sleep(10 * time.Millisecond)
		m.SetScanButtonActive(false)
		m.UpdateDiscordButtonState(true)
	}()
}

func (m *Manager) RefreshInventorySummaryDisplay() {
	summary := m.unifiedInventory.GetSummary()
	var summaryText strings.Builder
	summaryText.WriteString(fmt.Sprintf("Total unique items: %d\n", summary.TotalUniqueItems))
	summaryText.WriteString(fmt.Sprintf("Total items: %d\n", summary.TotalItems))
	summaryText.WriteString(fmt.Sprintf("Total wealth: %.2f HC (values from traderclub.gg)\n", summary.TotalWealth))
	summaryText.WriteString("------------------\n")

	for itemName, item := range summary.Items {
		summaryText.WriteString(fmt.Sprintf("%s: %d (%.2f HC)\n", itemName, item.Quantity, item.HCValue))
	}

	m.summaryText.SetText(summaryText.String())
}

func (m *Manager) RefreshInventoryIcons() {
	if m.iconContainer == nil {
		m.iconContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	}

	m.iconContainer.Objects = nil

	groupedItems := m.unifiedInventory.GetGroupedItems()
	for _, items := range groupedItems {
		btn := m.createGroupedItemButton(items)
		if btn != nil {
			m.iconContainer.Add(btn)
		}
	}

	m.iconContainer.Refresh()
}

func (m *Manager) RefreshTradingInventoryDisplay() {
	if m.tradeIconContainer == nil {
		m.tradeIconContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	}

	m.tradeIconContainer.Objects = nil

	groupedItems := m.unifiedInventory.GetGroupedItems()
	for _, items := range groupedItems {
		btn := m.createGroupedItemButton(items)
		m.tradeIconContainer.Add(btn)
	}

	m.tradeIconContainer.Refresh()

}

func (m *Manager) createGroupedItemButton(items []UnifiedItem) *widget.Button {
	if len(items) == 0 {
		return nil
	}

	btn := widget.NewButton("", func() {
		if m.tradeManager.IsTradeOpen() {
			m.showQuantityDialogInTradeManager(items)
		} else {
			m.displayItemInfo(items)
		}
	})

	btn.SetIcon(theme.AccountIcon())
	btn.Resize(fyne.NewSize(44, 44))

	go func() {
		iconURL := items[0].EnrichedItem.IconURL
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

func (m *Manager) createStyledContainerWithButtons(content fyne.CanvasObject, title string) *fyne.Container {
	// Create the background with rounded corners
	background := canvas.NewRectangle(color.NRGBA{R: 212, G: 221, B: 225, A: 255})
	background.StrokeColor = color.Black
	background.StrokeWidth = 1.35
	background.CornerRadius = 5

	titleText := canvas.NewText(title, color.Black)
	titleText.Alignment = fyne.TextAlignCenter
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	// Create the styled container
	styledContent := container.NewVBox(
		container.NewCenter(titleText),
		content,
	)

	paddedContent := container.NewVBox(
		container.NewPadded(styledContent),
	)

	styledContainer := container.NewMax(background, paddedContent)
	// Set minimum size to ensure coverage of the dialog area
	minSize := fyne.NewSize(400, 200) // Adjust the size as needed
	styledContainer.Resize(minSize)

	return container.NewMax(styledContainer)
}

func (m *Manager) addItemsToTrade(items []UnifiedItem, quantity int) {
	go func() {
		if len(items) == 0 {
			return
		}

		addedItems := make(map[int]inventory.Item)
		addedCount := 0

		for _, item := range items {
			for i := 0; i < item.Quantity && addedCount < quantity; i++ {
				if !m.tradeManager.IsTradeOpen() {
					break
				}
				if !item.InTrade {
					m.tradeManager.Offer(item.Item.ItemId)
					m.unifiedInventory.UpdateItemTradeStatus(item.Item.ItemId, true)
					addedItems[item.Item.ItemId] = item.Item
					addedCount++
					time.Sleep(550 * time.Millisecond)
				}
			}
			if addedCount >= quantity {
				break
			}
		}

		m.RefreshInventorySummaryDisplay()
		m.RefreshInventoryIcons()
		m.RefreshTradingInventoryDisplay()
	}()
}

func (m *Manager) handleTradeUpdated(args trade.Args) {
	m.updateTradeOffers(trading.Offers{
		Trader: args.Offers[0],
		Tradee: args.Offers[1],
	})
}

func (m *Manager) handleTradeAccepted(args trade.AcceptArgs) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update only the button states
	if m.isCurrentUser(args.Name) {
		m.tradeAcceptButton.Disable()
	}
}

func (m *Manager) ResetTradeManagerContent() {
	if m.tradeManagerWindow == nil {
		return
	}

	m.tradeOfferContainer.Objects = nil
	m.otherOfferContainer.Objects = nil
	m.yourTradeOffer = make(map[string]int)
	m.theirTradeOffer = make(map[string]int)

	m.tradeNewEntry.SetText("")
	m.tradeAcceptButton.Enable()

	m.tradeOfferContainer.Refresh()
	m.otherOfferContainer.Refresh()
	m.tradeManagerWindow.Content().Refresh()
}

func (m *Manager) ResetTradeManagerWindow() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset all trade-related data
	m.yourTradeOffer = make(map[string]int)
	m.theirTradeOffer = make(map[string]int)

	// Clear containers
	if m.tradeOfferContainer != nil {
		m.tradeOfferContainer.Objects = nil
	}
	if m.otherOfferContainer != nil {
		m.otherOfferContainer.Objects = nil
	}

	// Reset trade summary
	if m.tradeNewEntry != nil {
		m.tradeNewEntry.SetText("")
	}

	// Enable accept button
	if m.tradeAcceptButton != nil {
		m.tradeAcceptButton.Enable()
	}

	// Recreate the entire trade manager window content
	if m.tradeManagerWindow != nil {
		content := m.createTradeManagerWindowContent()
		m.tradeManagerWindow.SetContent(content)
		m.tradeManagerWindow.Content().Refresh()
	}
}

func (m *Manager) handleTradeCompleted(args trade.Args) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastTrade = trading.Offers{
		Trader: args.Offers[0],
		Tradee: args.Offers[1],
	}

	var ourOffer, theirOffer trade.Offer
	if m.isCurrentUser(args.Offers[0].Name) {
		ourOffer = args.Offers[0]
		theirOffer = args.Offers[1]
	} else {
		ourOffer = args.Offers[1]
		theirOffer = args.Offers[0]
	}

	// Remove traded items from our inventory
	for _, item := range ourOffer.Items {
		m.unifiedInventory.RemoveItem(item.ItemId)
	}

	// Add received items to our inventory
	for _, item := range theirOffer.Items {
		m.unifiedInventory.AddItem(item)
	}

	m.ResetTradeManagerContent()

	if m.tradeNewEntry != nil {
		m.tradeNewEntry.SetText("Trade completed successfully.")
	}

	m.RefreshInventorySummaryDisplay()
	m.RefreshInventoryIcons()
	m.RefreshTradingInventoryDisplay()
}

func (m *Manager) handleTradeClosed(args trade.Args) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset trade status for all items
	for itemId := range m.unifiedInventory.Items {
		m.unifiedInventory.UpdateItemTradeStatus(itemId, false)
	}

	m.ResetTradeManagerContent()

	if m.tradeNewEntry != nil {
		m.tradeNewEntry.SetText("Trade window closed. Awaiting Trade...")
	}

	m.RefreshInventoryIcons()
	m.RefreshTradingInventoryDisplay()
}

func (m *Manager) updateTradeOffers(offers trading.Offers) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.tradeOfferContainer == nil || m.otherOfferContainer == nil {
		return
	}

	m.tradeOfferContainer.Objects = nil
	m.otherOfferContainer.Objects = nil

	m.yourTradeOffer = make(map[string]int)
	m.theirTradeOffer = make(map[string]int)

	var myOffer, theirOffer trade.Offer
	var myName, theirName string
	var myTotalHC, theirTotalHC float64

	if m.isCurrentUser(offers.Trader.Name) {
		myOffer = offers.Trader
		theirOffer = offers.Tradee
		myName = offers.Trader.Name
		theirName = offers.Tradee.Name
	} else {
		myOffer = offers.Tradee
		theirOffer = offers.Trader
		myName = offers.Tradee.Name
		theirName = offers.Trader.Name
	}

	for _, item := range myOffer.Items {
		btn := m.createItemButton(item, true)
		m.tradeOfferContainer.Add(btn)

		enrichedItem := common.EnrichInventoryItem(item)
		m.yourTradeOffer[enrichedItem.Name]++
		myTotalHC += enrichedItem.HCValue
	}

	for _, item := range theirOffer.Items {
		btn := m.createItemButton(item, true)
		m.otherOfferContainer.Add(btn)

		enrichedItem := common.EnrichInventoryItem(item)
		m.theirTradeOffer[enrichedItem.Name]++
		theirTotalHC += enrichedItem.HCValue
	}

	m.tradeOfferContainer.Refresh()
	m.otherOfferContainer.Refresh()

	// Update the trade summary
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("%s's offer (%.2f HC):\n", myName, myTotalHC))
	for item, quantity := range m.yourTradeOffer {
		summary.WriteString(fmt.Sprintf("%s [%d]\n", item, quantity))
	}
	summary.WriteString(fmt.Sprintf("\n%s's offer (%.2f HC):\n", theirName, theirTotalHC))
	for item, quantity := range m.theirTradeOffer {
		summary.WriteString(fmt.Sprintf("%s [%d]\n", item, quantity))
	}

	if m.tradeNewEntry != nil {
		m.tradeNewEntry.SetText(summary.String())
	}

	if m.tradeManagerWindow != nil {
		m.tradeManagerWindow.Content().Refresh()
	}
}

func (m *Manager) createItemButton(item inventory.Item, inTrade bool) *widget.Button {
	btn := widget.NewButton("", func() {
		if !inTrade {
			m.showQuantityDialogInTradeManager([]UnifiedItem{{Item: item, EnrichedItem: common.EnrichInventoryItem(item), Quantity: 1}})
		}
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

func (m *Manager) RefreshTradeManagerWindow() {
	if m.tradeManagerWindow != nil {
		m.tradeManagerWindow.Content().Refresh()

		// Recreate the trade manager window content
		content := m.createTradeManagerWindowContent()
		m.tradeManagerWindow.SetContent(content)
	}
}

func (m *Manager) isCurrentUser(name string) bool {
	return m.profileManager.Profile.Name == name
}

func (m *Manager) UpdateRoomDisplay(objects map[int]room.Object, items map[int]room.Item) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.roomSummaryText.SetText(common.GetRoomSummary(objects, items))

	// Clear and repopulate room icons
	m.roomIconContainer.Objects = nil

	// Repopulate room icons
	for _, obj := range objects {
		m.addObjectIcon(obj)
	}
	for _, item := range items {
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

func (m *Manager) UpdateRoomDisplayAfterPickup() {
	m.UpdateRoomDisplayLock.Lock()
	defer m.UpdateRoomDisplayLock.Unlock()

	m.UpdateRoomDisplay(m.roomManager.Objects, m.roomManager.Items)
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

	tabs := NewCustomTabContainer(m, "Inventory", "Room")
	m.scanButton = tabs.scanButton
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
	roomItemsScroll.SetMinSize(fyne.NewSize(0, 125)) // Adjust this value to make it longer vertically

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
				selectedItemIds = make([]int, 0, len(objects))

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
				m.roomActionButton.SetActive(true)
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
				selectedItemIds = make([]int, 0, len(items))

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
				m.roomActionButton.SetActive(true)
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

	roomSummaryContainer := m.createStyledMultiLineEntryContainer(m.roomSummaryText, "Room Summary")
	roomItemsContainer := m.createStyledContainerWithButtons(roomItemsScroll, "Room Items")
	roomIdContainer := m.createStyledMultiLineEntryContainer(m.roomText, "Room Item IDs")

	m.roomActionButton = newCustomPickupButton(loadRoomActionIcon(), func() {
		if len(selectedItemIds) > 0 {
			m.roomActionButton.SetActive(true)
			go func() {
				m.pickupManager.PickupItems(selectedItemIds, func() {
					m.roomActionButton.SetActive(false)
					m.UpdateRoomDisplayAfterPickup()
				})
			}()
		}
	})

	actionButtonContainer := container.NewHBox(layout.NewSpacer(), m.roomActionButton, layout.NewSpacer())

	m.updateRoomDisplayFunc = updateRoomDisplay

	return container.NewVBox(
		roomSummaryContainer,
		roomItemsContainer,
		roomIdContainer,
		actionButtonContainer,
	)
}

func (m *Manager) createInventoryWindow() fyne.Window {
	inventoryWindow := m.app.NewWindow("Unified Inventory")
	inventoryWindow.SetIcon(m.window.Icon())

	if m.iconContainer == nil {
		m.iconContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	}

	itemsScroll := container.NewScroll(container.NewPadded(container.NewPadded(container.NewPadded(m.iconContainer))))
	itemsScroll.SetMinSize(fyne.NewSize(36, 36*8)) // Set the minimum rows visible to 30

	styledContainer := m.createStyledContainerWithButtons(itemsScroll, "Inventory Items")

	inventoryWindow.SetContent(styledContainer)
	inventoryWindow.Resize(fyne.NewSize(300, 400))

	inventoryWindow.SetCloseIntercept(func() {
		inventoryWindow.Hide()
	})

	return inventoryWindow
}

func (m *Manager) ShowInventoryWindow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inventoryWindow == nil {
		m.inventoryWindow = m.createInventoryWindow()
	}
	m.inventoryWindow.Show()
}

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
			b.label.SetText("Scanning...")
		} else {
			b.icon.Resource = loadScanIconInactive()
			b.label.SetText("Scan")
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
	tabs       []*customTab
	selected   int
	content    fyne.CanvasObject
	OnChanged  func()
	scanButton *customScanButton
	manager    *Manager
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

	c.scanButton = newCustomScanButton(loadScanIconInactive(), func() {
		manager.summaryText.SetText("") // Clear the summary text
		if manager.scanCallback != nil {
			manager.scanCallback()
		}
	})
	c.scanButton.SetActive(false)

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
}

func (r *customTabContainerRenderer) Refresh() {
	for i, tab := range r.container.tabs {
		tab.selected = (i == r.container.selected)
		tab.Refresh()
	}
}

func (r *customTabContainerRenderer) Objects() []fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, len(r.container.tabs)+1)
	for i, tab := range r.container.tabs {
		objects[i] = tab
	}
	objects[len(objects)-1] = r.container.scanButton
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
		return color.NRGBA{R: 192, G: 192, B: 192, A: 255}
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
		return color.NRGBA{R: 103, G: 148, B: 167, A: 255} // Set default color
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
		return 6
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

func (m *Manager) displayItemInfo(items []UnifiedItem) {
	if len(items) == 0 {
		return
	}

	var details strings.Builder
	representative := items[0]

	details.WriteString(fmt.Sprintf("Item name: %s [%d]\n", representative.EnrichedItem.Name, len(items)))
	details.WriteString(fmt.Sprintf("Total HC value: %.2f\n", representative.EnrichedItem.HCValue*float64(len(items))))
	details.WriteString("Item IDs:\n")

	for _, item := range items {
		details.WriteString(fmt.Sprintf("%d\n", item.Item.ItemId))
	}

	m.inventoryText.SetText(details.String())
}

func (m *Manager) showTradeConfirmationDialog() {
	var myOfferSummary, theirOfferSummary strings.Builder

	// Summarize your offer
	if len(m.yourTradeOffer) == 0 {
		myOfferSummary.WriteString("Nothing")
	} else {
		for item, quantity := range m.yourTradeOffer {
			myOfferSummary.WriteString(fmt.Sprintf("%d %s\n", quantity, item))
		}
	}

	// Summarize their offer
	if len(m.theirTradeOffer) == 0 {
		theirOfferSummary.WriteString("Nothing")
	} else {
		for item, quantity := range m.theirTradeOffer {
			theirOfferSummary.WriteString(fmt.Sprintf("%d %s\n", quantity, item))
		}
	}

	content := container.NewVBox(
		widget.NewLabel("You are about to trade:"),
		widget.NewLabel(myOfferSummary.String()),
		widget.NewLabel("\nYou will receive:"),
		widget.NewLabel(theirOfferSummary.String()),
	)

	dialogContent := m.createStyledContainerWithButtons(content, "Confirmation")

	dialog.ShowCustomConfirm("", "Confirm", "Cancel",
		dialogContent,
		func(confirmed bool) {
			if confirmed {
				m.tradeManager.Accept()
			}
			// If not confirmed, do nothing and return to main UI
		},
		m.tradeManagerWindow, // Anchor to the trade manager window
	)
}

func (m *Manager) createStyledScrollContainer(content fyne.CanvasObject, title string) *fyne.Container {
	background := canvas.NewRectangle(color.NRGBA{R: 212, G: 221, B: 225, A: 255})
	background.StrokeColor = color.Black
	background.StrokeWidth = 1.35
	background.CornerRadius = 5

	titleText := canvas.NewText(title, color.Black)
	titleText.Alignment = fyne.TextAlignCenter
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	scrollContainer := container.NewScroll(content)
	scrollContainer.SetMinSize(fyne.NewSize(0, 150)) // Adjust the height as needed

	contentWithTitle := container.NewBorder(
		container.NewCenter(titleText),
		nil, nil, nil,
		scrollContainer,
	)

	return container.NewMax(background, container.NewPadded(contentWithTitle))
}

func (m *Manager) createStyledMultiLineEntryContainer(content *widget.Entry, title string) *fyne.Container {
	background := canvas.NewRectangle(color.NRGBA{R: 212, G: 221, B: 225, A: 255})
	background.StrokeColor = color.Black
	background.StrokeWidth = 1.35
	background.CornerRadius = 5

	titleText := canvas.NewText(title, color.Black)
	titleText.Alignment = fyne.TextAlignCenter
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	contentWithTitle := container.NewBorder(
		container.NewCenter(titleText),
		nil, nil, nil,
		content,
	)

	return container.NewMax(background, container.NewPadded(contentWithTitle))
}
