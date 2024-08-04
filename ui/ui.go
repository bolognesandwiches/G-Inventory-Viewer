package ui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
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
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/inventory"
	"xabbo.b7c.io/goearth/shockwave/out"
	"xabbo.b7c.io/goearth/shockwave/profile"
	"xabbo.b7c.io/goearth/shockwave/room"
	"xabbo.b7c.io/goearth/shockwave/trade"
)

const (
	AssetServerBaseURL = "https://raw.githubusercontent.com/bolognesandwiches/G-Inventory-Viewer/master/assets/"
	DiscordWebhookURL  = "https://discord.com/api/webhooks/1261268478195798107/gyEDyPWQjRLwrH3cyw1hqKG4vKvs9h26lqlB-LriQs2RAgJgFz2IoAkEtG9Zct856Xec"
	APIBaseURL         = "https://gitemviewer.fly.dev"
	apiKey             = ""
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
	roomMutex                  sync.RWMutex
	roomItems                  map[int]room.Item
	roomSummaryMu              sync.RWMutex
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
	roomActionButton           *widget.Button
	tradeAcceptButton          *widget.Button
	scanCallback               func()
	inventorySummaryForDiscord string
	updateRoomDisplayFunc      func(map[int]room.Object, map[int]room.Item)
	UpdateRoomDisplayLock      sync.Mutex
	ext                        *g.Ext
	lastTrade                  trading.Offers
	tradeNewContainer          *fyne.Container
	tradeNewEntry              *widget.Entry
	yourTradeOffer             map[string]int
	theirTradeOffer            map[string]int
	unifiedInventory           *UnifiedInventory
	inventoryPopout            *fyne.Container // Changed from inventoryWindow to inventoryPopout
	tradeManagerWindow         fyne.Window
	quantityDialog             *dialog.CustomDialog
	tradeLog                   []TradeLogEntry
	tradeLogMutex              sync.Mutex
	tradeLogWindow             fyne.Window
	tradeLogEntry              *widget.Entry
	tradeLogVisible            bool
	tradeManagerPopout         *fyne.Container
	tradeLogPopout             *fyne.Container
	roomSummary                *RoomSummary
	roomToolsPopout            *fyne.Container
	currentCapture             *RoomCapture
	inventoryReportEntry       *widget.Entry
	profileSearchPopout        *fyne.Container
	profileSearchEntry         *widget.Entry
	profileSearchResultText    *widget.Entry
	currentRoomInfo            *room.Info
	scanEnabled                bool
	tradeCompleted             bool
	tradeMutex                 sync.Mutex
}

func NewUnifiedInventory() *UnifiedInventory {
	return &UnifiedInventory{
		Items: make(map[int]UnifiedItem),
		Summary: InventorySummary{
			Items: make(map[string]InventorySummaryItem),
		},
	}
}

func (m *Manager) showCustomDialog(title string, content fyne.CanvasObject, callback func(bool), parent fyne.Window) {
	// Fetch the GIF data
	gifURL := AssetServerBaseURL + "redlamp.gif"
	resp, err := http.Get(gifURL)
	if err != nil {
		dialog.ShowCustomConfirm(title, "Confirm", "Cancel", content, callback, parent)
		return
	}
	defer resp.Body.Close()

	gifData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		dialog.ShowCustomConfirm(title, "Confirm", "Cancel", content, callback, parent)
		return
	}

	// Decode the GIF
	gifImage, err := gif.DecodeAll(bytes.NewReader(gifData))
	if err != nil {
		dialog.ShowCustomConfirm(title, "Confirm", "Cancel", content, callback, parent)
		return
	}

	// Create an image widget
	img := canvas.NewImageFromImage(gifImage.Image[0])
	img.FillMode = canvas.ImageFillOriginal
	img.SetMinSize(fyne.NewSize(100, 100))

	// Create a container for the GIF with fixed size
	gifContainer := container.NewMax(img)
	gifContainer.Resize(fyne.NewSize(100, 100))

	// Animate the GIF
	go func() {
		for {
			for i := range gifImage.Image {
				img.Image = gifImage.Image[i]
				canvas.Refresh(img)
				time.Sleep(time.Duration(gifImage.Delay[i]*10) * time.Millisecond)
			}
		}
	}()

	// Create content with GIF and provided content
	dialogContent := container.NewVBox(
		container.NewCenter(gifContainer),
		content,
	)

	// Create custom dialog
	customDialog := dialog.NewCustomConfirm(title, "Confirm", "Cancel", dialogContent, callback, parent)

	customDialog.Show()
}

func (m *Manager) showQuantityDialogInTradeManager(items []UnifiedItem) {
	if len(items) == 0 || m.tradeManagerPopout == nil {
		return
	}

	representative := items[0]
	maxQuantity := 0
	for _, item := range items {
		if !item.InTrade {
			maxQuantity += item.Quantity
		}
	}

	message := fmt.Sprintf("Enter quantity for %s (max %d):", representative.EnrichedItem.Name, maxQuantity)

	quantityEntry := widget.NewEntry()
	quantityEntry.SetPlaceHolder("Enter Quantity...")

	// Create a container with a border
	entryWithBorder := container.NewBorder(nil, nil, nil, nil,
		canvas.NewRectangle(color.NRGBA{R: 100, G: 100, B: 100, A: 255}), // Border color
		container.NewPadded(quantityEntry),
	)
	entryWithBorder.Resize(fyne.NewSize(200, 40)) // Increase size

	dialogContent := container.NewVBox(
		widget.NewLabel(message),
		entryWithBorder,
	)

	m.showCustomDialog("", dialogContent, func(confirmed bool) {
		if confirmed {
			quantity, err := strconv.Atoi(quantityEntry.Text)
			if err != nil || quantity <= 0 || quantity > maxQuantity {
				dialog.ShowError(errors.New("Please enter a valid number between 1 and "+strconv.Itoa(maxQuantity)), m.window)
				return
			}
			m.addItemsToTrade(items, quantity)

			progress := dialog.NewProgress("Adding Items", "Adding items to trade...", m.window)
			go func() {
				for i := 0; i < quantity; i++ {
					progress.SetValue(float64(i+1) / float64(quantity))
					time.Sleep(550 * time.Millisecond)
				}
				progress.Hide()
			}()
		}
	}, m.window)
}

type TradeLogEntry struct {
	ID               int
	Date             string
	Trader           string
	Tradee           string
	ItemsTraded      []string
	ItemIDsTraded    []int
	HCValuesTraded   []float64
	ItemsReceived    []string
	ItemIDsReceived  []int
	HCValuesReceived []float64
}

func (m *Manager) ToggleTradeLogPopout() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.tradeLogPopout.Visible() {
		m.tradeLogPopout.Hide()
		m.window.Resize(fyne.NewSize(800, 600)) // Resize to original state when hidden
	} else {
		m.tradeLogPopout.Show()
		m.window.Resize(fyne.NewSize(800, 800)) // Resize to show the trade log popout
		m.updateTradeLogUI()                    // Update the trade log UI when showing the popout
	}
	m.window.Content().Refresh()
}

func (m *Manager) ShowTradeLogWindow() {
	if m.tradeLogPopout == nil {
		m.tradeLogPopout = m.createTradeLogContent()
	}
	m.ToggleTradeLogPopout()
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
	if m.tradeManagerPopout == nil {
		m.tradeManagerPopout = m.createTradeManagerContent()
	}
	if !m.tradeManagerPopout.Visible() { // Only toggle if it's not already visible
		m.ToggleTradeManagerPopout()
	}
}

func (m *Manager) ToggleTradeManagerPopout() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.tradeManagerPopout.Visible() {
		m.tradeManagerPopout.Hide()
	} else {
		m.tradeManagerPopout.Show()
	}
	m.window.Content().Refresh()
}

func (m *Manager) ToggleInventoryPopout() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.inventoryPopout.Visible() {
		m.inventoryPopout.Hide()
	} else {
		m.inventoryPopout.Show()
	}
	m.window.Content().Refresh()
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

func NewManager(app fyne.App, ext *g.Ext, invManager *inventory.Manager, roomManager *room.Manager, scanCallback func(), profileManager *profile.Manager) *Manager {
	m := &Manager{
		app:               app,
		ext:               ext,
		inventoryManager:  invManager,
		roomManager:       roomManager,
		scanCallback:      scanCallback,
		profileManager:    profileManager,
		tradeManager:      trading.NewManager(ext, profileManager, invManager),
		tradeNewContainer: container.NewGridWrap(fyne.NewSize(36, 36)),
		yourTradeOffer:    make(map[string]int),
		theirTradeOffer:   make(map[string]int),
		unifiedInventory:  NewUnifiedInventory(),
		iconContainer:     container.NewGridWrap(fyne.NewSize(36, 36)),
		roomSummaryMu:     sync.RWMutex{},
		scanEnabled:       false,
		tradeCompleted:    false,
		tradeMutex:        sync.Mutex{},
	}

	// Initialize speaker
	err := speaker.Init(44100, 44100/10)
	if err != nil {
		log.Println("Error initializing speaker:", err)
	}

	// Register handlers for inventory changes
	invManager.Updated(func() {
		currentItems := invManager.Items()
		if len(m.unifiedInventory.Items) == 0 {
			m.handleInitialInventoryUpdate(currentItems)
		} else {
			for id, item := range currentItems {
				if !m.unifiedInventory.ItemExists(id) {
					m.HandleItemAddition(item)
				}
			}
		}
	})

	invManager.ItemRemoved(func(args inventory.ItemArgs) {
		m.HandleItemRemoval(args.Item)
	})

	// Register trade event handlers
	m.tradeManager.Updated(m.handleTradeUpdated)
	m.tradeManager.Accepted(m.handleTradeAccepted)
	m.tradeManager.Completed(m.handleTradeCompleted)
	m.tradeManager.Closed(m.handleTradeClosed)

	m.roomSummary = NewRoomSummary()

	m.roomManager.ObjectAdded(func(args room.ObjectArgs) {
		m.roomMutex.Lock()
		defer m.roomMutex.Unlock()
		m.AddItemToRoom(args.Object)
		m.UpdateRoomDisplay(m.roomManager.Objects, m.roomManager.Items)
	})

	m.roomManager.ObjectRemoved(func(args room.ObjectArgs) {
		m.roomMutex.Lock()
		defer m.roomMutex.Unlock()
		m.RemoveItemFromRoom(args.Object.Id)
		m.UpdateRoomDisplay(m.roomManager.Objects, m.roomManager.Items)
	})

	m.roomManager.ObjectsLoaded(func(args room.ObjectsArgs) {
		m.roomMutex.Lock()
		defer m.roomMutex.Unlock()
		m.UpdateRoomDisplay(m.roomManager.Objects, m.roomManager.Items)
	})

	m.roomManager.ItemsLoaded(func(args room.ItemsArgs) {
		m.roomMutex.Lock()
		defer m.roomMutex.Unlock()
		m.UpdateRoomDisplay(m.roomManager.Objects, m.roomManager.Items)

		if m.currentRoomInfo != nil {
			go m.SendRoomScanData(*m.currentRoomInfo, m.roomManager.Objects, m.roomManager.Items)
		}
	})

	roomManager.Entered(func(args room.Args) {
		m.roomMutex.Lock()
		defer m.roomMutex.Unlock()
		m.currentRoomInfo = args.Info
		if m.currentRoomInfo != nil {
			// Log or handle the room entry
		}
	})

	return m
}

func (m *Manager) AddItemToRoom(item room.Object) {
	m.roomSummary.mu.Lock()
	m.roomSummary.Items[item.Id] = item
	m.roomSummary.mu.Unlock()

	// Update room display asynchronously
	go m.UpdateRoomDisplay(m.roomManager.Objects, m.roomManager.Items)
}

func (m *Manager) RemoveItemFromRoom(itemId int) {
	m.roomSummary.mu.Lock()
	delete(m.roomSummary.Items, itemId)
	m.roomSummary.mu.Unlock()

	// Update room display asynchronously
	go m.UpdateRoomDisplay(m.roomManager.Objects, m.roomManager.Items)
}

func (m *Manager) UpdateRoomSummaryDisplay() {
	// Update the room summary display in the UI
	// This will depend on how you're currently displaying the room summary
	// For example:
	summary := common.GetRoomSummary(m.roomSummary.Items, nil)
	m.roomSummaryText.SetText(summary)
}

func (m *Manager) handleInitialInventoryUpdate(items map[int]inventory.Item) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range items {
		m.unifiedInventory.AddItem(item)
	}
	m.RefreshInventorySummaryDisplay()
	m.RefreshInventoryIcons()
}

func (ui *UnifiedInventory) ItemExists(itemId int) bool {
	ui.mu.RLock()
	defer ui.mu.RUnlock()
	_, exists := ui.Items[itemId]
	return exists
}

func (m *Manager) HandleItemAddition(item inventory.Item) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.unifiedInventory.AddItem(item)
	m.RefreshInventorySummaryDisplay()
	m.RefreshInventoryIcons()
}

func (m *Manager) HandleItemRemoval(item inventory.Item) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.unifiedInventory.RemoveItem(item.ItemId)
	m.RefreshInventorySummaryDisplay()
	m.RefreshInventoryIcons()
}

func (m *Manager) UpdateInventoryDisplay(items map[int]inventory.Item) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing items and summary
	m.unifiedInventory = NewUnifiedInventory()

	for _, item := range items {
		m.unifiedInventory.AddItem(item)
	}

	// Send scan data to server
	err := m.SendInventoryScanData(items)
	if err != nil {
		// Handle error (e.g., show a dialog to the user)
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
	background := canvas.NewRectangle(color.NRGBA{R: 42, G: 42, B: 42, A: 255})
	background.StrokeColor = color.NRGBA{R: 180, G: 180, B: 180, A: 255}
	background.StrokeWidth = 3
	background.CornerRadius = 5

	titleText := canvas.NewText(title, color.NRGBA{R: 180, G: 180, B: 180, A: 255})
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
	// Determine if the trade is opened by checking if both offers are empty
	opened := len(args.Offers[0].Items) == 0 && len(args.Offers[1].Items) == 0
	if opened {
		m.HandleTradeStatus(true)
		go m.playSound("opened.wav")
	}
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
	if m.tradeOfferContainer != nil {
		m.tradeOfferContainer.Objects = nil
		m.tradeOfferContainer.Refresh()
	}
	if m.otherOfferContainer != nil {
		m.otherOfferContainer.Objects = nil
		m.otherOfferContainer.Refresh()
	}
	m.yourTradeOffer = make(map[string]int)
	m.theirTradeOffer = make(map[string]int)

	if m.tradeNewEntry != nil {
		m.tradeNewEntry.SetText("")
	}
	if m.tradeAcceptButton != nil {
		m.tradeAcceptButton.Disable() // Explicitly disable the accept trade button
	}

	if m.tradeManagerPopout != nil {
		m.tradeManagerPopout.Refresh()
	}
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
	m.tradeMutex.Lock()
	m.tradeCompleted = true
	m.tradeMutex.Unlock()

	go m.playSound("completed.wav")

	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastTrade = trading.Offers{
		Trader: args.Offers[0],
		Tradee: args.Offers[1],
	}

	var ourOffer, theirOffer trade.Offer
	var ourName, theirName string
	var myTotalHC, theirTotalHC float64
	if m.isCurrentUser(args.Offers[0].Name) {
		ourOffer = args.Offers[0]
		theirOffer = args.Offers[1]
		ourName = args.Offers[0].Name
		theirName = args.Offers[1].Name
	} else {
		ourOffer = args.Offers[1]
		theirOffer = args.Offers[0]
		ourName = args.Offers[1].Name
		theirName = args.Offers[0].Name
	}

	var myOfferItems, theirOfferItems []string
	var myOfferItemIDs, theirOfferItemIDs []int
	var myOfferHCValues, theirOfferHCValues []float64

	// Process our offer
	for _, item := range ourOffer.Items {
		enrichedItem := common.EnrichInventoryItem(item)
		myOfferItems = append(myOfferItems, enrichedItem.Name)
		myOfferItemIDs = append(myOfferItemIDs, item.ItemId)
		myOfferHCValues = append(myOfferHCValues, enrichedItem.HCValue)
		myTotalHC += enrichedItem.HCValue
	}

	// Process their offer
	for _, item := range theirOffer.Items {
		enrichedItem := common.EnrichInventoryItem(item)
		theirOfferItems = append(theirOfferItems, enrichedItem.Name)
		theirOfferItemIDs = append(theirOfferItemIDs, item.ItemId)
		theirOfferHCValues = append(theirOfferHCValues, enrichedItem.HCValue)
		theirTotalHC += enrichedItem.HCValue
	}

	// Add received items to our inventory before logging
	for _, item := range theirOffer.Items {
		m.unifiedInventory.AddItem(item)
	}

	// Generate the next trade ID
	tradeID := len(m.tradeLog) + 1

	// Log the trade internally
	logEntry := TradeLogEntry{
		ID:               tradeID,
		Date:             time.Now().Format("02-01-2006 15:04:05"),
		Trader:           ourName,
		Tradee:           theirName,
		ItemsTraded:      myOfferItems,
		ItemIDsTraded:    myOfferItemIDs,
		HCValuesTraded:   myOfferHCValues,
		ItemsReceived:    theirOfferItems,
		ItemIDsReceived:  theirOfferItemIDs,
		HCValuesReceived: theirOfferHCValues,
	}

	m.tradeLogMutex.Lock()
	m.tradeLog = append(m.tradeLog, logEntry)
	m.tradeLogMutex.Unlock()

	// If the Trade Log popout is open, update it
	if m.tradeLogPopout.Visible() {
		m.updateTradeLogUI()
	}

	// Send trade log to API
	if m.scanEnabled {
		go func() {
			err := m.SendTradeLogToAPI(logEntry)
			if err != nil {
				// Handle error
			}
		}()
	}

	// Remove traded items from our inventory
	for _, item := range ourOffer.Items {
		m.unifiedInventory.RemoveItem(item.ItemId)
	}

	m.ResetTradeManagerContent()

	if m.tradeNewEntry != nil {
		m.tradeNewEntry.SetText("Trade completed successfully.")
	}

	m.RefreshInventorySummaryDisplay()
	m.RefreshInventoryIcons()
	m.RefreshTradingInventoryDisplay()

	if m.tradeAcceptButton != nil {
		m.tradeAcceptButton.Enable() // Re-enable the accept trade button after trade completion
	}

	m.tradeManagerPopout.Hide() // Hide the trade manager popout when the trade is completed
	m.window.Content().Refresh()

	// Reset the tradeCompleted flag after a delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		m.tradeMutex.Lock()
		m.tradeCompleted = false
		m.tradeMutex.Unlock()
	}()
}
func (m *Manager) handleTradeClosed(args trade.Args) {
	go func() {
		// Wait a short time to see if the trade was completed
		time.Sleep(100 * time.Millisecond)

		m.tradeMutex.Lock()
		wasCompleted := m.tradeCompleted
		m.tradeMutex.Unlock()

		if !wasCompleted {
			m.playSound("closed.wav")
		}

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

		if m.tradeAcceptButton != nil {
			m.tradeAcceptButton.Enable() // Re-enable the accept trade button after trade closure
		}

		m.tradeManagerPopout.Hide() // Hide the trade manager popout when the trade is closed
		m.window.Content().Refresh()
	}()
}

func (m *Manager) updateTradeLogUI() {
	m.tradeLogMutex.Lock()
	defer m.tradeLogMutex.Unlock()

	var logText strings.Builder

	// Helper function to truncate and pad strings
	formatStr := func(str string, length int) string {
		if len(str) > length {
			return str[:length-3] + "..."
		}
		return str + strings.Repeat(" ", length-len(str))
	}

	// Add header
	header := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s",
		formatStr("ID", 5),
		formatStr("Date", 20),
		formatStr("Trader", 15),
		formatStr("Recipient", 15),
		formatStr("Item", 20),
		formatStr("Item ID", 10),
		formatStr("HC Value", 10))
	logText.WriteString(header + "\n")
	logText.WriteString(strings.Repeat("-", 100) + "\n")

	// Sort trades in descending order
	sortedTrades := make([]TradeLogEntry, len(m.tradeLog))
	copy(sortedTrades, m.tradeLog)
	sort.Slice(sortedTrades, func(i, j int) bool {
		return sortedTrades[i].ID > sortedTrades[j].ID
	})

	for _, entry := range sortedTrades {
		// Write traded items
		for i := range entry.ItemsTraded {
			line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%d\t%.2f",
				formatStr(fmt.Sprintf("%d", entry.ID), 5),
				formatStr(entry.Date, 20),
				formatStr(entry.Trader, 15),
				formatStr(entry.Tradee, 15),
				formatStr(entry.ItemsTraded[i], 20),
				abs(entry.ItemIDsTraded[i]),
				entry.HCValuesTraded[i])
			logText.WriteString(line + "\n")
		}
		// Write received items
		for i := range entry.ItemsReceived {
			line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%d\t%.2f",
				formatStr(fmt.Sprintf("%d", entry.ID), 5),
				formatStr(entry.Date, 20),
				formatStr(entry.Tradee, 15),
				formatStr(entry.Trader, 15),
				formatStr(entry.ItemsReceived[i], 20),
				abs(entry.ItemIDsReceived[i]),
				entry.HCValuesReceived[i])
			logText.WriteString(line + "\n")
		}
		// Add a spacer between trades
		logText.WriteString(strings.Repeat("-", 120) + "\n")
	}

	m.tradeLogEntry.SetText(logText.String())
	m.tradeLogEntry.Refresh()
}

// Helper function to truncate long strings

func (m *Manager) exportTradeLogToCSV() {
	var csvContent strings.Builder
	csvContent.WriteString("ID,Date,Trader,Recipient,Item,Item ID,HC Value\n")

	for _, entry := range m.tradeLog {
		for i := range entry.ItemsTraded {
			csvContent.WriteString(fmt.Sprintf("%d,%s,%s,%s,%s,%d,%.2f\n",
				entry.ID, entry.Date, entry.Trader, entry.Tradee,
				entry.ItemsTraded[i], entry.ItemIDsTraded[i], entry.HCValuesTraded[i]))
		}
		for i := range entry.ItemsReceived {
			csvContent.WriteString(fmt.Sprintf("%d,%s,%s,%s,%s,%d,%.2f\n",
				entry.ID, entry.Date, entry.Tradee, entry.Trader,
				entry.ItemsReceived[i], entry.ItemIDsReceived[i], entry.HCValuesReceived[i]))
		}
	}

	// Write to file
	filename := fmt.Sprintf("trade_log_%s.csv", time.Now().Format("20060102_150405"))
	err := ioutil.WriteFile(filename, []byte(csvContent.String()), 0644)
	if err != nil {
		dialog.ShowError(err, m.window) // Ensure the dialog is anchored to a window
	} else {
		dialog.ShowInformation("Export Successful", "Trade log exported successfully.", m.window) // Ensure the dialog is anchored to a window
	}
}

func (ui *UnifiedInventory) GetItemByName(name string) *common.EnrichedInventoryItem {
	ui.mu.RLock()
	defer ui.mu.RUnlock()

	for _, item := range ui.Items {
		if item.EnrichedItem.Name == name {
			return &item.EnrichedItem
		}
	}
	return nil
}

func (m *Manager) createTradeLogContent() *fyne.Container {
	m.tradeLogEntry = widget.NewMultiLineEntry()
	m.tradeLogEntry.Wrapping = fyne.TextWrapWord
	m.tradeLogEntry.SetPlaceHolder("Trade Log")
	m.tradeLogEntry.SetMinRowsVisible(10)

	// Set the font to a monospaced font for consistent alignment
	m.tradeLogEntry.TextStyle = fyne.TextStyle{Monospace: false}

	exportButton := widget.NewButton("Export to CSV", func() {
		m.exportTradeLogToCSV()
	})

	tradeLogContainer := m.createStyledMultiLineEntryContainer(m.tradeLogEntry, "Trade Log")

	content := container.NewVBox(
		tradeLogContainer,
		exportButton,
	)

	popout := container.NewVBox(
		content,
	)
	popout.Hide() // Ensure it's hidden initially
	return popout
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

	if m.tradeAcceptButton != nil && m.tradeManagerPopout.Visible() {
		m.tradeAcceptButton.Enable()
	}

	if m.tradeManagerPopout != nil {
		m.tradeManagerPopout.Refresh()
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

type RoomSummary struct {
	Items map[int]room.Object
	mu    sync.RWMutex
}

func NewRoomSummary() *RoomSummary {
	return &RoomSummary{
		Items: make(map[int]room.Object),
	}
}

func (m *Manager) isCurrentUser(name string) bool {
	return m.profileManager.Profile.Name == name
}

func (m *Manager) UpdateRoomDisplay(objects map[int]room.Object, items map[int]room.Item) {
	m.roomSummary.mu.Lock()
	defer m.roomSummary.mu.Unlock()

	m.roomSummary.Items = objects

	if m.updateRoomDisplayFunc != nil {
		// Consider running this in a goroutine if it's a long-running operation
		go m.updateRoomDisplayFunc(objects, items)
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

	// Create the header (image or fallback text)
	header := m.createHeader("teleport.png")

	// Setup the inventory content
	inventoryContent := m.setupInventoryContent()

	// Main container with header and inventory content
	mainContainer := container.NewBorder(
		header,
		nil, nil, nil,
		inventoryContent,
	)

	m.inventoryPopout = m.createInventoryContent()
	m.inventoryPopout.Hide()

	m.tradeManagerPopout = m.createTradeManagerContent()
	m.tradeManagerPopout.Hide()

	m.tradeLogPopout = m.createTradeLogContent()
	m.tradeLogPopout.Hide()

	m.roomToolsPopout = m.createRoomToolsContent()
	m.roomToolsPopout.Hide()

	m.profileSearchPopout = m.createProfileSearchContent()
	m.profileSearchPopout.Hide()

	// Create a horizontal container for InventoryPopout and ProfileSearchPopout
	popoutsContainer := container.NewHBox(
		m.inventoryPopout,
		m.profileSearchPopout,
	)

	// Create the main layout
	mainLayout := container.NewBorder(
		nil,
		container.NewVBox(m.tradeLogPopout, m.roomToolsPopout),
		m.tradeManagerPopout,
		popoutsContainer, // Replace the individual popouts with the horizontal container
		mainContainer,
	)

	// Create the bordered container
	borderedContainer := NewBorderedContainer(mainLayout, 4) // Using scale factor 4

	m.window.SetContent(borderedContainer)
	m.window.Resize(fyne.NewSize(300, 600))
	m.window.SetPadded(false)

	m.window.ShowAndRun()
}

func (m *Manager) createHeader(imagePath string) fyne.CanvasObject {
	header, err := NewImageHeader(imagePath)
	if err != nil {
		// Fallback to a text header if image loading fails
		return widget.NewLabel("Inventory")
	}

	// Wrap the header in a container with fixed height
	return container.NewStack(
		header,
		layout.NewSpacer(), // This will make the container expand to fill available space
		container.NewWithoutLayout(header),
	)
}

func (m *Manager) createTradeManagerContent() *fyne.Container {
	if m.tradeOfferContainer == nil {
		m.tradeOfferContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	}
	if m.otherOfferContainer == nil {
		m.otherOfferContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	}

	m.tradeNewEntry = widget.NewMultiLineEntry()
	m.tradeNewEntry.Wrapping = fyne.TextWrapWord
	m.tradeNewEntry.SetPlaceHolder("Awaiting trade...")
	m.tradeNewEntry.SetMinRowsVisible(8)

	tradeSummaryContainer := m.createStyledMultiLineEntryContainer(m.tradeNewEntry, "Trade Summary")

	// Wrap the scroll containers in a container with a fixed size
	tradeOfferScroll := m.createStyledScrollContainer(m.tradeOfferContainer, "Your Offer")
	tradeOfferFixedSize := container.NewVBox(
		tradeOfferScroll,
		layout.NewSpacer(),
	)
	tradeOfferFixedSize.Resize(fyne.NewSize(200, 100)) // Set the fixed size for 'Your Offer'

	otherOfferScroll := m.createStyledScrollContainer(m.otherOfferContainer, "Their Offer")
	otherOfferFixedSize := container.NewVBox(
		otherOfferScroll,
		layout.NewSpacer(),
	)
	otherOfferFixedSize.Resize(fyne.NewSize(200, 100)) // Set the fixed size for 'Their Offer'

	m.tradeAcceptButton = widget.NewButton("Accept Trade", func() {
		m.showTradeConfirmationDialog()
	})

	tradeLogButton := widget.NewButton("Trade Log", func() {
		m.ShowTradeLogWindow()
	})

	actionButtonContainer := container.NewHBox(
		layout.NewSpacer(),
		m.tradeAcceptButton,
		tradeLogButton,
		layout.NewSpacer(),
	)

	content := container.NewVBox(
		tradeSummaryContainer,
		tradeOfferFixedSize,
		otherOfferFixedSize,
		actionButtonContainer,
	)

	styledContainer := m.createStyledContainerWithButtons(content, "Trade Manager")

	popout := container.NewVBox(
		styledContainer,
		layout.NewSpacer(), // Ensures the minimum height
	)

	// Ensure the popout respects the minimum size
	popoutMinSize := fyne.NewSize(200, 375)
	popout.Resize(popoutMinSize) // Ensure the popout respects the minimum size

	return container.NewMax(popout)
}

func (m *Manager) createInventoryContent() *fyne.Container {
	if m.iconContainer == nil {
		m.iconContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	}

	itemsScroll := container.NewScroll(container.NewPadded(container.NewPadded(container.NewPadded(m.iconContainer))))
	itemsScroll.SetMinSize(fyne.NewSize(200, 375))

	styledContainer := m.createStyledContainerWithButtons(itemsScroll, "Inventory Items")

	popout := container.NewVBox(
		styledContainer,
	)

	return popout
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

func (m *Manager) ShowInventoryWindow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inventoryPopout == nil {
		m.inventoryPopout = m.createInventoryContent()
	}
	m.inventoryPopout.Show()
	m.window.Content().Refresh()
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

func (b *customScanButton) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(color.NRGBA{R: 212, G: 221, B: 225, A: 255})
	background.StrokeColor = color.Black
	background.StrokeWidth = 3
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
	text.TextSize = 13

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
	r.text.Color = color.NRGBA{R: 128, G: 128, B: 128, A: 255}
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

type habboTheme struct{}

func (m *habboTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 42, G: 42, B: 42, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 180, G: 180, B: 180, A: 255} // Lighter font color
	case theme.ColorNameButton:
		return color.NRGBA{R: 100, G: 100, B: 100, A: 255} // Darker button color
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 42, G: 42, B: 42, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 140, G: 140, B: 140, A: 255} // Slightly lighter placeholder text
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 42, G: 42, B: 42, A: 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 42, G: 42, B: 42, A: 255}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 99, G: 192, B: 127, A: 255}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 42, G: 42, B: 42, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 252, G: 100, B: 52, A: 255}
	case theme.ColorNameFocus:
		return color.White
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 99, G: 192, B: 127, A: 255}
	default:
		return color.NRGBA{R: 42, G: 42, B: 42, A: 255}
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
		return 13
	case theme.SizeNamePadding:
		return 8
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
	for item, quantity := range m.yourTradeOffer {
		myOfferSummary.WriteString(fmt.Sprintf("%d x %s\n", quantity, item))
	}
	if myOfferSummary.Len() == 0 {
		myOfferSummary.WriteString("Nothing")
	}

	// Summarize their offer
	for item, quantity := range m.theirTradeOffer {
		theirOfferSummary.WriteString(fmt.Sprintf("%d x %s\n", quantity, item))
	}
	if theirOfferSummary.Len() == 0 {
		theirOfferSummary.WriteString("Nothing")
	}

	message := fmt.Sprintf("You are about to trade:\n%s\n\nYou will receive:\n%s",
		myOfferSummary.String(), theirOfferSummary.String())

	content := widget.NewLabel(message)
	content.Wrapping = fyne.TextWrapWord

	m.showCustomDialog("", content, func(confirmed bool) {
		if confirmed {
			m.tradeManager.Accept()
		}
	}, m.window)
}
func (m *Manager) createStyledScrollContainer(content fyne.CanvasObject, title string) *fyne.Container {
	background := canvas.NewRectangle(color.NRGBA{R: 42, G: 42, B: 42, A: 255})
	background.StrokeColor = color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	background.StrokeWidth = 3
	background.CornerRadius = 5

	titleText := canvas.NewText(title, color.NRGBA{R: 180, G: 180, B: 180, A: 255})
	titleText.Alignment = fyne.TextAlignCenter
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.Color = color.NRGBA{R: 180, G: 180, B: 180, A: 255}

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
	background := canvas.NewRectangle(color.NRGBA{R: 42, G: 42, B: 42, A: 255})
	background.StrokeColor = color.NRGBA{R: 180, G: 180, B: 180, A: 255}
	background.StrokeWidth = 3
	background.CornerRadius = 5

	titleText := canvas.NewText(title, color.NRGBA{R: 180, G: 180, B: 180, A: 255})
	titleText.Alignment = fyne.TextAlignCenter
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	contentWithTitle := container.NewBorder(
		container.NewCenter(titleText),
		nil, nil, nil,
		content,
	)

	return container.NewMax(background, container.NewPadded(contentWithTitle))
}

func (m *Manager) HandleTradeStatus(opened bool) {
	if opened {
		m.ShowTradeManagerWindow()
	}
}

type RoomCapture struct {
	FloorItems map[string][]FloorItemInfo
	WallItems  map[string][]WallItemInfo
	Timestamp  time.Time
}

type FloorItemInfo struct {
	Name      string
	Width     int
	Height    int
	X         int
	Y         int
	Direction int
}

type WallItemInfo struct {
	Name          string
	PlacementInfo string
}

func (m *Manager) CaptureCurrentRoom() {
	m.mu.Lock()
	defer m.mu.Unlock()

	capture := &RoomCapture{
		FloorItems: make(map[string][]FloorItemInfo),
		WallItems:  make(map[string][]WallItemInfo),
		Timestamp:  time.Now(),
	}

	// Capture floor items (Objects)
	for _, obj := range m.roomManager.Objects {
		enrichedObj := common.EnrichRoomObject(obj)
		floorInfo := FloorItemInfo{
			Name:      enrichedObj.Name,
			Width:     obj.Width,
			Height:    obj.Height,
			X:         obj.X,
			Y:         obj.Y,
			Direction: obj.Direction,
		}
		capture.FloorItems[enrichedObj.Name] = append(capture.FloorItems[enrichedObj.Name], floorInfo)
	}

	// Capture wall items (Items)
	for _, item := range m.roomManager.Items {
		enrichedItem := common.EnrichRoomItem(item)
		wallInfo := WallItemInfo{
			Name:          enrichedItem.Name,
			PlacementInfo: item.Location,
		}
		capture.WallItems[enrichedItem.Name] = append(capture.WallItems[enrichedItem.Name], wallInfo)
	}

	m.currentCapture = capture

	dialog.ShowInformation("Room Captured", fmt.Sprintf("The current room layout has been captured.\nFloor Items: %d, Wall Items: %d", len(capture.FloorItems), len(capture.WallItems)), m.window)
}
func (m *Manager) ValidateInventoryForCapture() string {
	if m.currentCapture == nil {
		return "No room captured."
	}

	var report strings.Builder
	report.WriteString("Inventory Validation Report:\n\n")

	report.WriteString("Floor Items:\n")
	for itemName, itemInfos := range m.currentCapture.FloorItems {
		m.writeItemValidation(&report, itemName, len(itemInfos), false)
	}

	report.WriteString("\nWall Items:\n")
	for itemName, itemInfos := range m.currentCapture.WallItems {
		m.writeItemValidation(&report, itemName, len(itemInfos), true)
	}

	return report.String()
}

func (m *Manager) writeItemValidation(report *strings.Builder, itemName string, requiredCount int, isWallItem bool) {
	inventoryCount := m.getInventoryItemCount(itemName, isWallItem)

	report.WriteString(fmt.Sprintf("%s:\n", itemName))
	report.WriteString(fmt.Sprintf("  Required: %d\n", requiredCount))
	report.WriteString(fmt.Sprintf("  In Inventory: %d\n", inventoryCount))

	if inventoryCount >= requiredCount {
		report.WriteString("  Status: ✓ Sufficient\n")
	} else {
		report.WriteString(fmt.Sprintf("  Status: ✗ Missing %d\n", requiredCount-inventoryCount))
	}
	report.WriteString("\n")
}

func (m *Manager) getInventoryItemCount(itemName string, isWallItem bool) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, item := range m.unifiedInventory.Items {
		enrichedItem := common.EnrichInventoryItem(item.Item)
		if enrichedItem.Name == itemName && (isWallItem == (item.Item.Type == "I")) {
			count += item.Quantity
		}
	}
	return count
}

func (m *Manager) DuplicateRoom(capture *RoomCapture) error {
	if capture == nil {
		return errors.New("no room capture available")
	}

	if !m.canPlaceItemsInCurrentRoom() {
		return errors.New("cannot place items in the current room")
	}

	progress := dialog.NewProgress("Duplicating Room", "Placing items...", m.window)
	progress.Show()
	defer progress.Hide()

	totalItems := m.getTotalItemCount(capture)
	placedItems := 0

	// Place "Petal Patch" first if it exists
	if petalPatches, exists := capture.FloorItems["Petal Patch"]; exists {
		for _, info := range petalPatches {
			if !m.placeItem("Petal Patch", false, info) {
				return fmt.Errorf("failed to place Petal Patch")
			}
			placedItems++
			progress.SetValue(float64(placedItems) / float64(totalItems))
		}
		delete(capture.FloorItems, "Petal Patch")
	}

	// Place remaining floor items
	for itemName, itemInfos := range capture.FloorItems {
		for _, info := range itemInfos {
			if !m.placeItem(itemName, false, info) {
				return fmt.Errorf("failed to place floor item: %s", itemName)
			}
			placedItems++
			progress.SetValue(float64(placedItems) / float64(totalItems))
		}
	}

	// Place wall items
	for itemName, itemInfos := range capture.WallItems {
		for _, info := range itemInfos {
			if !m.placeItem(itemName, true, info) {
				return fmt.Errorf("failed to place wall item: %s", itemName)
			}
			placedItems++
			progress.SetValue(float64(placedItems) / float64(totalItems))
		}
	}

	m.RefreshInventorySummaryDisplay()
	m.RefreshInventoryIcons()
	m.RefreshTradingInventoryDisplay()
	m.UpdateRoomDisplayAfterPickup()

	return nil
}

func (m *Manager) getTotalItemCount(capture *RoomCapture) int {
	total := 0
	for _, itemInfos := range capture.FloorItems {
		total += len(itemInfos)
	}
	for _, itemInfos := range capture.WallItems {
		total += len(itemInfos)
	}
	return total
}

func (m *Manager) placeItem(itemName string, isWallItem bool, info interface{}) bool {

	m.roomMutex.Lock()
	defer m.roomMutex.Unlock()
	var itemToPlace *inventory.Item
	var itemId int
	for id, unifiedItem := range m.unifiedInventory.Items {
		enrichedItem := common.EnrichInventoryItem(unifiedItem.Item)
		if enrichedItem.Name == itemName && (isWallItem == (unifiedItem.Item.Type == "I")) {
			itemToPlace = &unifiedItem.Item
			itemId = id
			break
		}
	}

	if itemToPlace == nil {
		return false
	}

	var packetData string

	if isWallItem {
		wallInfo := info.(WallItemInfo)
		packetData = fmt.Sprintf("%d %s", itemToPlace.ItemId, wallInfo.PlacementInfo)
	} else {
		floorInfo := info.(FloorItemInfo)
		packetData = fmt.Sprintf("%d %d %d %d %d %d", itemToPlace.ItemId, floorInfo.X, floorInfo.Y, floorInfo.Width, floorInfo.Height, floorInfo.Direction)
	}

	m.ext.Send(out.PLACESTUFF, []byte(packetData))

	// Instead of removing the item, decrease its quantity
	unifiedItem := m.unifiedInventory.Items[itemId]
	unifiedItem.Quantity--
	if unifiedItem.Quantity <= 0 {
		m.unifiedInventory.RemoveItem(itemId)
	} else {
		m.unifiedInventory.Items[itemId] = unifiedItem
	}

	time.Sleep(600 * time.Millisecond)

	return true
}

func (m *Manager) PickupItems(itemIds []int, onComplete func()) {

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range itemIds {
		var packetData string
		var itemToAdd inventory.Item

		if roomObj, exists := m.roomManager.Objects[id]; exists {
			packetData = fmt.Sprintf("new stuff %d", id)
			itemToAdd = inventory.Item{
				ItemId: -abs(id), // Ensure floor item ID is negative
				Type:   "S",
				Class:  roomObj.Class,
			}
			delete(m.roomManager.Objects, id)
		} else if roomItem, exists := m.roomManager.Items[id]; exists {
			packetData = fmt.Sprintf("new item %d", id)
			itemToAdd = inventory.Item{
				ItemId: id, // Keep wall item ID as is (positive)
				Type:   "I",
				Class:  roomItem.Class,
				Props:  roomItem.Type,
			}
			delete(m.roomManager.Items, id)
		} else {
			continue
		}

		m.ext.Send(out.ADDSTRIPITEM, []byte(packetData))

		// Add the item to the unified inventory
		m.unifiedInventory.AddItem(itemToAdd)

		time.Sleep(600 * time.Millisecond)
	}

	if onComplete != nil {
		onComplete()
	}

	m.RefreshInventorySummaryDisplay()
	m.RefreshInventoryIcons()
	m.RefreshTradingInventoryDisplay()
	m.UpdateRoomDisplayAfterPickup()
}

// Helper function to get absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func (m *Manager) ImportRoomLayout(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	capture := &RoomCapture{
		FloorItems: make(map[string][]FloorItemInfo),
		WallItems:  make(map[string][]WallItemInfo),
		Timestamp:  time.Now(),
	}

	var currentItem string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Name: ") {
			currentItem = strings.TrimPrefix(line, "Name: ")
			continue
		}
		if strings.HasPrefix(line, "Count: ") || strings.HasPrefix(line, "HC Value: ") {
			continue
		}
		if strings.HasPrefix(line, "Item Details:") {
			continue
		}
		if strings.Contains(line, "Location: ") {
			parts := strings.Split(line, "Location: ")
			if len(parts) == 2 {
				wallInfo := WallItemInfo{
					Name:          currentItem,
					PlacementInfo: parts[1],
				}
				capture.WallItems[currentItem] = append(capture.WallItems[currentItem], wallInfo)
			}
		} else if strings.Contains(line, "(W:") {
			parts := strings.Split(line, "(")
			if len(parts) == 2 {
				details := strings.Trim(parts[1], ")")
				var x, y, w, h, dir int
				fmt.Sscanf(details, "W:%d, H:%d, X:%d, Y:%d, Dir:%d", &w, &h, &x, &y, &dir)
				floorInfo := FloorItemInfo{
					Name:      currentItem,
					Width:     w,
					Height:    h,
					X:         x,
					Y:         y,
					Direction: dir,
				}
				capture.FloorItems[currentItem] = append(capture.FloorItems[currentItem], floorInfo)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	m.currentCapture = capture
	return nil
}
func (m *Manager) canPlaceItemsInCurrentRoom() bool {
	// Implement logic to check if the current room allows item placement
	// This might involve checking room ownership or permissions
	// For now, we'll return true as a placeholder
	return true
}

func (m *Manager) createRoomToolsContent() *fyne.Container {
	// Room Summary content
	m.roomText = widget.NewMultiLineEntry()
	m.roomText.Wrapping = fyne.TextWrapWord
	m.roomText.SetPlaceHolder("Room Item IDs")
	m.roomText.SetMinRowsVisible(5)

	m.roomSummaryText = widget.NewMultiLineEntry()
	m.roomSummaryText.Wrapping = fyne.TextWrapWord
	m.roomSummaryText.SetPlaceHolder("Room Summary")
	m.roomSummaryText.SetMinRowsVisible(5)

	m.roomIconContainer = container.NewGridWrap(fyne.NewSize(36, 36))
	roomItemsScroll := container.NewScroll(container.NewPadded(container.NewPadded(container.NewPadded(m.roomIconContainer))))
	roomItemsScroll.SetMinSize(fyne.NewSize(0, 125))

	var selectedItemIds []int

	updateRoomDisplay := func(objects map[int]room.Object, items map[int]room.Item) {
		m.roomSummary.mu.Lock()
		defer m.roomSummary.mu.Unlock()

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
				m.roomActionButton.Enable()
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
				m.roomActionButton.Enable()
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

		// Update the room summary text
		m.roomSummaryText.SetText(common.GetRoomSummary(objects, items))

		var itemIDs strings.Builder
		for id := range objects {
			itemIDs.WriteString(fmt.Sprintf("%d\n", id))
		}
		for id := range items {
			itemIDs.WriteString(fmt.Sprintf("%d\n", id))
		}
		m.roomText.SetText(itemIDs.String())
	}

	m.updateRoomDisplayFunc = updateRoomDisplay

	roomSummaryContainer := m.createStyledMultiLineEntryContainer(m.roomSummaryText, "Room Summary")
	roomItemsContainer := m.createStyledContainerWithButtons(roomItemsScroll, "Room Items")
	roomIdContainer := m.createStyledMultiLineEntryContainer(m.roomText, "Room Item IDs")

	// Create a horizontal container for Room Items and Room Item IDs
	roomItemsAndIdsContainer := NewCustomSplit(Horizontal,
		roomItemsContainer,
		roomIdContainer,
	)
	roomItemsAndIdsContainer.SetOffset(0.6) // Adjust this value to change the relative sizes

	m.roomActionButton = widget.NewButton("Pick Up Items", func() {
		if len(selectedItemIds) > 0 {
			go func() {
				m.PickupItems(selectedItemIds, func() {
					m.UpdateRoomDisplayAfterPickup()
				})
			}()
		}
	})
	m.roomActionButton.Disable()

	// Room Duplicator content
	importButton := widget.NewButton("Import", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, m.window)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			err = m.ImportRoomLayout(reader.URI().Path())
			if err != nil {
				dialog.ShowError(err, m.window)
				return
			}
			dialog.ShowInformation("Success", "Room layout imported successfully", m.window)
			m.updateRoomToolsContent()
		}, m.window)
	})

	exportButton := widget.NewButton("Export", func() {
		if m.currentCapture == nil {
			dialog.ShowError(errors.New("no room captured"), m.window)
			return
		}
		dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, m.window)
				return
			}
			if writer == nil {
				return
			}
			defer writer.Close()

			err = m.ExportRoomLayout(writer)
			if err != nil {
				dialog.ShowError(err, m.window)
				return
			}
			dialog.ShowInformation("Success", "Room layout exported successfully", m.window)
		}, m.window)
	})

	captureButton := widget.NewButton("Capture", func() {
		m.CaptureCurrentRoom()
		m.updateRoomToolsContent()
	})

	validateButton := widget.NewButton("Validate", func() {
		if m.currentCapture == nil {
			dialog.ShowError(errors.New("no room captured"), m.window)
			return
		}
		m.ValidateInventoryForCapture()
		m.updateRoomToolsContent()
	})

	duplicateButton := widget.NewButton("Duplicate", func() {
		if m.currentCapture == nil {
			dialog.ShowError(errors.New("no room captured"), m.window)
			return
		}
		go func() {
			err := m.DuplicateRoom(m.currentCapture)
			if err != nil {
				dialog.ShowError(err, m.window)
			} else {
				dialog.ShowInformation("Success", "Room duplication completed", m.window)
			}
		}()
	})

	buttonSize := fyne.NewSize(80, 30)
	importButton.Resize(buttonSize)
	exportButton.Resize(buttonSize)
	captureButton.Resize(buttonSize)
	validateButton.Resize(buttonSize)
	duplicateButton.Resize(buttonSize)
	m.roomActionButton.Resize(buttonSize)

	buttonsContainer := container.NewHBox(
		layout.NewSpacer(),
		importButton,
		exportButton,
		captureButton,
		validateButton,
		duplicateButton,
		m.roomActionButton,
		layout.NewSpacer(),
	)

	m.inventoryReportEntry = widget.NewMultiLineEntry()
	m.inventoryReportEntry.SetText("Capture a room and validate inventory to see the report.")
	m.inventoryReportEntry.Wrapping = fyne.TextWrapWord
	m.inventoryReportEntry.SetMinRowsVisible(5)

	inventoryReportContainer := m.createStyledMultiLineEntryContainer(m.inventoryReportEntry, "Inventory Validation")

	content := container.NewVBox(
		roomSummaryContainer,
		roomItemsAndIdsContainer,
		buttonsContainer,
		inventoryReportContainer,
	)

	return m.createStyledContainerWithButtons(content, "Room Tools")
}

type Orientation int

const (
	Vertical Orientation = iota
	Horizontal
)

func (m *Manager) updateRoomToolsContent() {
	if m.inventoryReportEntry != nil {
		m.inventoryReportEntry.SetText(m.ValidateInventoryForCapture())
	}
}

func (m *Manager) ExportRoomLayout(writer io.Writer) error {
	if m.currentCapture == nil {
		return errors.New("no room captured")
	}

	for itemName, wallItems := range m.currentCapture.WallItems {
		fmt.Fprintf(writer, "Name: %s\n", itemName)
		fmt.Fprintf(writer, "Count: %d\n", len(wallItems))
		fmt.Fprintf(writer, "HC Value: 0.00\n") // You might want to calculate the actual HC value
		fmt.Fprintln(writer, "Item Details:")
		for _, item := range wallItems {
			fmt.Fprintf(writer, "(Location: %s\n", item.PlacementInfo)
		}
		fmt.Fprintln(writer)
	}

	for itemName, floorItems := range m.currentCapture.FloorItems {
		fmt.Fprintf(writer, "Name: %s\n", itemName)
		fmt.Fprintf(writer, "Count: %d\n", len(floorItems))
		fmt.Fprintf(writer, "HC Value: 0.00\n") // You might want to calculate the actual HC value
		fmt.Fprintln(writer, "Item Details:")
		for _, item := range floorItems {
			fmt.Fprintf(writer, "(W:%d, H:%d, X:%d, Y:%d, Dir:%d\n", item.Width, item.Height, item.X, item.Y, item.Direction)
		}
		fmt.Fprintln(writer)
	}

	return nil
}

func (m *Manager) ToggleRoomToolsPopout() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.roomToolsPopout == nil {
		m.roomToolsPopout = m.createRoomToolsContent()
	}

	if m.roomToolsPopout.Visible() {
		m.roomToolsPopout.Hide()
		m.window.Resize(fyne.NewSize(800, 600)) // Resize to original state when hidden
	} else {
		m.roomToolsPopout.Show()
		m.window.Resize(fyne.NewSize(800, 800)) // Resize to show the room tools popout
		m.updateRoomToolsContent()
	}
	m.window.Content().Refresh()
}

type CustomSplit struct {
	widget.BaseWidget
	orientation Orientation
	offset      float64
	Leading     fyne.CanvasObject
	Trailing    fyne.CanvasObject
	minSize     fyne.Size
}

func (c *CustomSplit) Visible() bool {
	return c.offset > 0 && c.offset < 1
}

func NewCustomSplit(orientation Orientation, leading, trailing fyne.CanvasObject) *CustomSplit {
	split := &CustomSplit{
		orientation: orientation,
		offset:      0.5,
		Leading:     leading,
		Trailing:    trailing,
	}
	split.ExtendBaseWidget(split)
	split.updateMinSize()
	return split
}

func (c *CustomSplit) updateMinSize() {
	if c.orientation == Horizontal {
		c.minSize = fyne.NewSize(
			c.Leading.MinSize().Width+c.Trailing.MinSize().Width,
			fyne.Max(c.Leading.MinSize().Height, c.Trailing.MinSize().Height),
		)
	} else {
		c.minSize = fyne.NewSize(
			fyne.Max(c.Leading.MinSize().Width, c.Trailing.MinSize().Width),
			c.Leading.MinSize().Height+c.Trailing.MinSize().Height,
		)
	}
}

func (c *CustomSplit) Resize(size fyne.Size) {
	c.BaseWidget.Resize(size)
	c.updateChildrenPositions()
}

func (c *CustomSplit) updateChildrenPositions() {
	size := c.Size()
	if c.orientation == Horizontal {
		leadingWidth := int(float64(size.Width) * c.offset)
		c.Leading.Resize(fyne.NewSize(float32(leadingWidth), size.Height))
		c.Leading.Move(fyne.NewPos(0, 0))
		c.Trailing.Resize(fyne.NewSize(float32(int(size.Width)-leadingWidth), size.Height))
		c.Trailing.Move(fyne.NewPos(float32(leadingWidth), 0))
	} else {
		leadingHeight := int(float64(size.Height) * c.offset)
		c.Leading.Resize(fyne.NewSize(size.Width, float32(leadingHeight)))
		c.Leading.Move(fyne.NewPos(0, 0))
		c.Trailing.Resize(fyne.NewSize(size.Width, float32(int(size.Height)-leadingHeight)))
		c.Trailing.Move(fyne.NewPos(0, float32(leadingHeight)))
	}
}

func (c *CustomSplit) CreateRenderer() fyne.WidgetRenderer {
	return &customSplitRenderer{
		split:   c,
		divider: &canvas.Rectangle{FillColor: color.Transparent},
	}
}

type customSplitRenderer struct {
	split    *CustomSplit
	divider  *canvas.Rectangle
	lastSize fyne.Size
}

func (r *customSplitRenderer) MinSize() fyne.Size {
	return r.split.minSize
}

func (r *customSplitRenderer) Layout(size fyne.Size) {
	r.lastSize = size
	r.layoutObjects(size)
}

func (r *customSplitRenderer) layoutObjects(size fyne.Size) {
	if r.split.orientation == Horizontal {
		r.layoutHorizontal(size)
	} else {
		r.layoutVertical(size)
	}
}

func (c *CustomSplit) MinSize() fyne.Size {
	return c.minSize
}

func (r *customSplitRenderer) layoutHorizontal(size fyne.Size) {
	leadingWidth := int(float64(size.Width) * r.split.offset)
	r.split.Leading.Resize(fyne.NewSize(float32(leadingWidth), size.Height))
	r.split.Leading.Move(fyne.NewPos(0, 0))
	r.split.Trailing.Resize(fyne.NewSize(float32(int(size.Width)-leadingWidth), size.Height))
	r.split.Trailing.Move(fyne.NewPos(float32(leadingWidth), 0))
}

func (r *customSplitRenderer) layoutVertical(size fyne.Size) {
	leadingHeight := int(float64(size.Height) * r.split.offset)
	r.split.Leading.Resize(fyne.NewSize(size.Width, float32(leadingHeight)))
	r.split.Leading.Move(fyne.NewPos(0, 0))
	r.split.Trailing.Resize(fyne.NewSize(size.Width, float32(int(size.Height)-leadingHeight)))
	r.split.Trailing.Move(fyne.NewPos(0, float32(leadingHeight)))
}

func (r *customSplitRenderer) Refresh() {
	r.Layout(r.lastSize)
}

func (r *customSplitRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.split.Leading, r.split.Trailing, r.divider}
}

func (r *customSplitRenderer) Destroy() {}

func (c *CustomSplit) SetOffset(offset float64) {
	c.offset = offset
	c.Refresh()
}

type BorderedContainer struct {
	widget.BaseWidget
	Content fyne.CanvasObject

	topImage, bottomImage, leftImage, rightImage                   *canvas.Image
	topLeftImage, topRightImage, bottomLeftImage, bottomRightImage *canvas.Image

	scaleFactor float32
}

func NewBorderedContainer(content fyne.CanvasObject, scaleFactor float32) *BorderedContainer {
	c := &BorderedContainer{
		Content:     content,
		scaleFactor: scaleFactor,
	}
	c.ExtendBaseWidget(c)

	baseURL := AssetServerBaseURL + "silver/"

	// Load your images here
	c.topImage = mustLoadCanvasImage(baseURL + "top.png")
	c.bottomImage = mustLoadCanvasImage(baseURL + "bottom.png")
	c.leftImage = mustLoadCanvasImage(baseURL + "left.png")
	c.rightImage = mustLoadCanvasImage(baseURL + "right.png")
	c.topLeftImage = mustLoadCanvasImage(baseURL + "topleft.png")
	c.topRightImage = mustLoadCanvasImage(baseURL + "topright.png")
	c.bottomLeftImage = mustLoadCanvasImage(baseURL + "bottomleft.png")
	c.bottomRightImage = mustLoadCanvasImage(baseURL + "bottomright.png")

	return c
}

func mustLoadCanvasImage(url string) *canvas.Image {
	img, err := loadImageFromURL(url)
	if err != nil {
	}
	return canvas.NewImageFromImage(img)
}

func (c *BorderedContainer) CreateRenderer() fyne.WidgetRenderer {
	return &borderedContainerRenderer{
		container: c,
	}
}

type borderedContainerRenderer struct {
	container *BorderedContainer
}

func (r *borderedContainerRenderer) Destroy() {}

func (r *borderedContainerRenderer) Layout(size fyne.Size) {
	c := r.container
	borderSize := theme.Padding() * 2 * c.scaleFactor // Adjust border size based on scale factor

	// Corner images
	cornerSize := fyne.NewSize(borderSize, borderSize)
	c.topLeftImage.Resize(cornerSize)
	c.topLeftImage.Move(fyne.NewPos(0, 0))

	c.topRightImage.Resize(cornerSize)
	c.topRightImage.Move(fyne.NewPos(size.Width-borderSize, 0))

	c.bottomLeftImage.Resize(cornerSize)
	c.bottomLeftImage.Move(fyne.NewPos(0, size.Height-borderSize))

	c.bottomRightImage.Resize(cornerSize)
	c.bottomRightImage.Move(fyne.NewPos(size.Width-borderSize, size.Height-borderSize))

	// Side images
	c.topImage.Resize(fyne.NewSize(size.Width-2*borderSize, borderSize))
	c.topImage.Move(fyne.NewPos(borderSize, 0))

	c.bottomImage.Resize(fyne.NewSize(size.Width-2*borderSize, borderSize))
	c.bottomImage.Move(fyne.NewPos(borderSize, size.Height-borderSize))

	c.leftImage.Resize(fyne.NewSize(borderSize, size.Height-2*borderSize))
	c.leftImage.Move(fyne.NewPos(0, borderSize))

	c.rightImage.Resize(fyne.NewSize(borderSize, size.Height-2*borderSize))
	c.rightImage.Move(fyne.NewPos(size.Width-borderSize, borderSize))

	// Position main content
	contentSize := size.Subtract(fyne.NewSize(2*borderSize, 2*borderSize))
	c.Content.Resize(contentSize)
	c.Content.Move(fyne.NewPos(borderSize, borderSize))
}

func (r *borderedContainerRenderer) MinSize() fyne.Size {
	borderSize := theme.Padding() * 2 * r.container.scaleFactor
	return r.container.Content.MinSize().Add(fyne.NewSize(borderSize*2, borderSize*2))
}
func (r *borderedContainerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{
		r.container.Content,
		r.container.topImage, r.container.bottomImage, r.container.leftImage, r.container.rightImage,
		r.container.topLeftImage, r.container.topRightImage, r.container.bottomLeftImage, r.container.bottomRightImage,
	}
}

func (r *borderedContainerRenderer) Refresh() {
	r.Layout(r.container.Size())
	canvas.Refresh(r.container)
}

type ImageHeader struct {
	widget.BaseWidget
	image *canvas.Image
}

func NewImageHeader(imagePath string) (fyne.CanvasObject, error) {
	fullURL := AssetServerBaseURL + imagePath
	img, err := loadImageFromURL(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to load image: %v", err)
	}

	canvasImage := canvas.NewImageFromImage(img)
	canvasImage.FillMode = canvas.ImageFillOriginal
	canvasImage.ScaleMode = canvas.ImageScaleSmooth

	return canvasImage, nil
}

func (h *ImageHeader) CreateRenderer() fyne.WidgetRenderer {
	return &imageHeaderRenderer{header: h}
}

type imageHeaderRenderer struct {
	header *ImageHeader
}

func (r *imageHeaderRenderer) MinSize() fyne.Size {
	return r.header.image.MinSize()
}

func (r *imageHeaderRenderer) Layout(size fyne.Size) {
	r.header.image.Resize(size)
}

func (r *imageHeaderRenderer) Refresh() {
	canvas.Refresh(r.header.image)
}

func (r *imageHeaderRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.header.image}
}

func (r *imageHeaderRenderer) Destroy() {}

func loadImageFromURL(url string) (image.Image, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func (m *Manager) setupInventoryContent() fyne.CanvasObject {
	m.inventoryText = widget.NewMultiLineEntry()
	m.inventoryText.Wrapping = fyne.TextWrapWord
	m.inventoryText.SetPlaceHolder("Open your Inventory and then click on Item icons to view more information.")
	m.inventoryText.SetMinRowsVisible(10)

	m.summaryText = widget.NewMultiLineEntry()
	m.summaryText.Wrapping = fyne.TextWrapWord
	m.summaryText.SetPlaceHolder("Click on 'Scan' to begin scanning your inventory!")
	m.summaryText.SetMinRowsVisible(10)

	summaryContainer := m.createStyledMultiLineEntryContainer(m.summaryText, "Inventory Summary")
	idContainer := m.createStyledMultiLineEntryContainer(m.inventoryText, "Item Details")

	scanButton := widget.NewButton("Scan", func() {
		if m.scanCallback != nil {
			m.scanCallback()
		}
	})
	openInventoryButton := widget.NewButton("Inventory", func() {
		m.ToggleInventoryPopout()
	})
	openTradeManagerButton := widget.NewButton("Trade Manager", func() {
		m.ToggleTradeManagerPopout()
	})
	openTradeLogButton := widget.NewButton("Trade Log", func() {
		m.ToggleTradeLogPopout()
	})
	openRoomToolsButton := widget.NewButton("Room Tools", func() {
		m.ToggleRoomToolsPopout()
	})
	openProfileSearchButton := widget.NewButton("Profile Search", func() {
		m.ToggleProfileSearchPopout()
	})

	buttonSize := fyne.NewSize(100, 30)
	scanButton.Resize(buttonSize)
	openInventoryButton.Resize(buttonSize)
	openTradeManagerButton.Resize(buttonSize)
	openTradeLogButton.Resize(buttonSize)
	openRoomToolsButton.Resize(buttonSize)
	openProfileSearchButton.Resize(buttonSize)

	buttonsContainer := container.NewHBox(
		layout.NewSpacer(),
		scanButton,
		openInventoryButton,
		openTradeManagerButton,
		openTradeLogButton,
		openRoomToolsButton,
		openProfileSearchButton,
		layout.NewSpacer(),
	)

	scanToggle := widget.NewCheck("Enable Data Sharing", func(enabled bool) {
		m.scanEnabled = enabled
	})
	scanToggle.SetChecked(false) // Default to opt-out

	deletionRequestButton := widget.NewButton("Request Data Deletion", func() {
		m.showDeletionRequestForm()
	})

	// Load and create the TC_Badge
	tcBadge, err := m.loadImage("TC_Badge.png")
	if err != nil {
	}

	var tcBadgeButton *widget.Button
	if tcBadge != nil {
		tcBadgeButton = widget.NewButton("", func() {
			url, _ := url.Parse("https://www.traderclub.gg")
			err := fyne.CurrentApp().OpenURL(url)
			if err != nil {
			}
		})
		tcBadgeButton.SetIcon(tcBadge)
		tcBadgeButton.Importance = widget.LowImportance
		tcBadgeButton.Resize(fyne.NewSize(50, 50)) // Adjust size as needed
	}

	// Create a horizontal container for the toggle, TC badge, and deletion request button
	toggleAndButtonContainer := container.NewHBox(
		scanToggle,
		layout.NewSpacer(), // This pushes the TC badge to the center
		tcBadgeButton,
		layout.NewSpacer(), // This pushes the deletion request button to the right
		deletionRequestButton,
	)

	// Create a background for the container
	background := canvas.NewRectangle(color.NRGBA{R: 60, G: 60, B: 60, A: 255})

	// Combine the background and content
	contentWithBackground := container.NewMax(
		background,
		container.NewPadded(toggleAndButtonContainer),
	)

	// Add a label below the container
	infoLabel := widget.NewLabel("When enabled, inventory and room data will be shared with the G-itemViewer API.")
	infoLabel.Alignment = fyne.TextAlignCenter

	toggleContainer := container.NewVBox(
		contentWithBackground,
		infoLabel,
	)

	return container.NewVBox(
		summaryContainer,
		idContainer,
		buttonsContainer,
		toggleContainer,
	)
}

func (m *Manager) showDeletionRequestForm() {
	reason := widget.NewMultiLineEntry()
	reason.SetPlaceHolder("Reason for deletion request")

	content := container.NewVBox(
		widget.NewLabel("Reason:"),
		reason,
	)

	customDialog := dialog.NewCustom("Data Deletion Request", "Cancel", content, m.window)

	submitButton := widget.NewButton("Submit", func() {
		go m.sendDeletionRequest(m.profileManager.Profile.Name, reason.Text)
		customDialog.Hide()
		dialog.ShowInformation("Request Submitted", "Your data deletion request has been submitted for review.", m.window)
	})

	cancelButton := widget.NewButton("Cancel", func() {
		customDialog.Hide()
	})

	customDialog.SetButtons([]fyne.CanvasObject{cancelButton, submitButton})
	customDialog.Show()
}
func (m *Manager) sendDeletionRequest(username, reason string) {
	requestData := map[string]string{
		"username": username,
		"reason":   reason,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		log.Printf("Error marshaling deletion request: %v", err)
		return
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/deletion-request", APIBaseURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
	}
}

// ScanPayload represents the payload structure for the scan API
type ScanPayload struct {
	UserID    string      `json:"user_id"`
	ScanType  string      `json:"scan_type"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

func SendScanPayload(userID, scanType string, data interface{}) error {
	scanData := ScanPayload{
		UserID:    userID,
		ScanType:  scanType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}

	jsonData, err := json.Marshal(scanData)
	if err != nil {
		return fmt.Errorf("failed to marshal scan data: %v", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/scan", APIBaseURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK response: %s", resp.Status)
	}

	return nil
}

type ProfileSearch struct {
	ProfileName string `json:"profile_name"`
}

type ProfileSearchResult struct {
	UserID    string          `json:"user_id"`
	ScanType  string          `json:"scan_type"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

func (m *Manager) createProfileSearchContent() *fyne.Container {
	m.profileSearchEntry = widget.NewEntry()
	m.profileSearchEntry.SetPlaceHolder("Enter Profile Name...")

	searchButton := widget.NewButton("Search", func() {
		m.performProfileSearch(m.profileSearchEntry.Text)
	})

	// Wrap the entry and button in a container and set the desired width
	entryContainer := container.NewVBox(
		m.profileSearchEntry,
		searchButton,
	)
	entryScroll := container.NewScroll(entryContainer)
	entryScroll.SetMinSize(fyne.NewSize(400, entryScroll.MinSize().Height+50)) // Set desired width and height

	m.profileSearchResultText = widget.NewMultiLineEntry()
	m.profileSearchResultText.Wrapping = fyne.TextWrapWord
	m.profileSearchResultText.SetPlaceHolder("Profile Search Results")
	m.profileSearchResultText.SetMinRowsVisible(20)

	resultContainer := m.createStyledMultiLineEntryContainer(m.profileSearchResultText, "Search Results")
	resultScroll := container.NewScroll(resultContainer)
	resultScroll.SetMinSize(fyne.NewSize(400, 200)) // Set desired width and height

	content := container.NewVBox(
		entryScroll,
		resultScroll,
	)

	styledContainer := m.createStyledContainerWithButtons(content, "Profile Search")

	popout := container.NewVBox(
		styledContainer,
	)
	popout.Hide() // Ensure it's hidden initially

	return popout
}

func (m *Manager) ToggleProfileSearchPopout() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.profileSearchPopout.Visible() {
		m.profileSearchPopout.Hide()
	} else {
		m.profileSearchPopout.Show()
	}

	m.window.Content().Refresh()
}
func (m *Manager) performProfileSearch(profileName string) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/scans/%s", APIBaseURL, profileName), nil)
	if err != nil {
		m.profileSearchResultText.SetText(fmt.Sprintf("Error creating request: %v", err))
		return
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		m.profileSearchResultText.SetText(fmt.Sprintf("Error sending request: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		m.profileSearchResultText.SetText(fmt.Sprintf("Received non-OK response: %s", resp.Status))
		return
	}

	var results []ProfileSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		m.profileSearchResultText.SetText(fmt.Sprintf("Error decoding response: %v", err))
		return
	}

	if len(results) == 0 {
		m.profileSearchResultText.SetText("No scan data found for the specified profile.")
		return
	}

	var summaryText strings.Builder

	// Group results by scan type
	inventoryScans := make([]ProfileSearchResult, 0)
	roomScans := make([]ProfileSearchResult, 0)

	for _, result := range results {
		if strings.EqualFold(result.ScanType, "inventory") {
			inventoryScans = append(inventoryScans, result)
		} else if strings.EqualFold(result.ScanType, "room") {
			roomScans = append(roomScans, result)
		}
	}

	// Process inventory scans
	if len(inventoryScans) > 0 {
		latestInventoryScan := inventoryScans[0]
		for _, scan := range inventoryScans {
			if scan.Timestamp > latestInventoryScan.Timestamp {
				latestInventoryScan = scan
			}
		}
		summaryText.WriteString(processInventoryScan(latestInventoryScan))
		summaryText.WriteString("\n\n")
	}

	// Process room scans
	for _, roomScan := range roomScans {
		summaryText.WriteString(processRoomScan(roomScan))
		summaryText.WriteString("\n\n")
	}

	m.profileSearchResultText.SetText(summaryText.String())
}

func processInventoryScan(scan ProfileSearchResult) string {
	var summaryText strings.Builder
	summaryText.WriteString(fmt.Sprintf("Last Inventory Scan Date: %s\n", scan.Timestamp))
	summaryText.WriteString("Scan Type: inventory\n")

	var data []map[string]interface{}
	if err := json.Unmarshal(scan.Data, &data); err != nil {
		return fmt.Sprintf("Error decoding inventory scan data: %v", err)
	}

	summary := make(map[string]int)
	totalItems := 0
	totalHCValue := 0.0

	for _, item := range data {
		name := item["name"].(string)
		summary[name]++
		totalItems++

		if hcVal, ok := item["hc_val"].(float64); ok {
			totalHCValue += hcVal
		}
	}

	summaryText.WriteString(fmt.Sprintf("Total Items: %d\n", totalItems))
	summaryText.WriteString(fmt.Sprintf("Total Unique Items: %d\n", len(summary)))
	summaryText.WriteString(fmt.Sprintf("Total HC Value: %.2f\n", totalHCValue))
	summaryText.WriteString("------------------\n")

	sortedItems := getSortedItems(summary, data)

	for _, item := range sortedItems {
		summaryText.WriteString(fmt.Sprintf("%s [%d] (%.2f HC)\n", item.Name, item.Count, item.HCValue))
	}

	return summaryText.String()
}

func processRoomScan(scan ProfileSearchResult) string {
	var summaryText strings.Builder
	summaryText.WriteString(fmt.Sprintf("Room Scan Date: %s\n", scan.Timestamp))
	summaryText.WriteString("Scan Type: room\n")

	var data struct {
		RoomID int                      `json:"room_id"`
		Items  []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(scan.Data, &data); err != nil {
		return fmt.Sprintf("Error decoding room scan data: %v", err)
	}

	summaryText.WriteString(fmt.Sprintf("Room ID: %d\n", data.RoomID))

	summary := make(map[string]int)
	totalItems := 0
	totalHCValue := 0.0

	for _, item := range data.Items {
		name := item["name"].(string)
		summary[name]++
		totalItems++

		if hcVal, ok := item["hc_val"].(float64); ok {
			totalHCValue += hcVal
		}
	}

	summaryText.WriteString(fmt.Sprintf("Total Items: %d\n", totalItems))
	summaryText.WriteString(fmt.Sprintf("Total Unique Items: %d\n", len(summary)))
	summaryText.WriteString(fmt.Sprintf("Total HC Value: %.2f\n", totalHCValue))
	summaryText.WriteString("------------------\n")

	sortedItems := getSortedItems(summary, data.Items)

	for _, item := range sortedItems {
		summaryText.WriteString(fmt.Sprintf("%s [%d] (%.2f HC)\n", item.Name, item.Count, item.HCValue))
	}

	return summaryText.String()
}

type ItemSummary struct {
	Name    string
	Count   int
	HCValue float64
}

func getSortedItems(summary map[string]int, data []map[string]interface{}) []ItemSummary {
	var sortedItems []ItemSummary

	for name, count := range summary {
		hcValue := 0.0
		for _, item := range data {
			if item["name"].(string) == name {
				if hcVal, ok := item["hc_val"].(float64); ok {
					hcValue += hcVal * float64(count)
				}
				break
			}
		}
		sortedItems = append(sortedItems, ItemSummary{Name: name, Count: count, HCValue: hcValue})
	}

	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].HCValue > sortedItems[j].HCValue
	})

	return sortedItems
}
func (m *Manager) SendRoomScanData(roomInfo room.Info, objects map[int]room.Object, items map[int]room.Item) error {
	if !m.scanEnabled {
		return nil
	}

	m.roomMutex.RLock()
	defer m.roomMutex.RUnlock()

	var roomItems []map[string]interface{}

	for id, obj := range objects {
		enrichedObj := common.EnrichRoomObject(obj)
		roomItems = append(roomItems, map[string]interface{}{
			"id":     id,
			"name":   enrichedObj.Name,
			"hc_val": enrichedObj.HCValue,
		})
	}

	for id, item := range items {
		enrichedItem := common.EnrichRoomItem(item)
		roomItems = append(roomItems, map[string]interface{}{
			"id":     id,
			"name":   enrichedItem.Name,
			"hc_val": enrichedItem.HCValue,
		})
	}

	payload := map[string]interface{}{
		"room_id": roomInfo.Id,
		"items":   roomItems,
	}

	// Release the lock before making the network call
	m.roomMutex.RUnlock()
	err := SendScanPayload(roomInfo.Owner, "room", payload)
	m.roomMutex.RLock()

	return err
}
func (m *Manager) SendInventoryScanData(items map[int]inventory.Item) error {
	if !m.scanEnabled {
		return nil
	}

	var inventoryItems []map[string]interface{}

	for _, item := range items {
		enrichedItem := common.EnrichInventoryItem(item)
		inventoryItems = append(inventoryItems, map[string]interface{}{
			"id":     item.ItemId,
			"name":   enrichedItem.Name,
			"hc_val": enrichedItem.HCValue,
		})
	}

	return SendScanPayload(m.profileManager.Profile.Name, "inventory", inventoryItems)
}

// Add this struct definition
type TradeItem struct {
	UID       string    `json:"uid"`
	Date      time.Time `json:"date"`
	Trader    string    `json:"trader"`
	Recipient string    `json:"recipient"`
	ItemName  string    `json:"item_name"`
	ItemID    int       `json:"item_id"`
	HCValue   float64   `json:"hc_value"`
}

func (m *Manager) SendTradeLogToAPI(trade TradeLogEntry) error {
	var tradeItems []TradeItem

	// Add traded items
	for i, itemName := range trade.ItemsTraded {
		tradeItems = append(tradeItems, TradeItem{
			Date:      time.Now(),
			Trader:    trade.Trader,
			Recipient: trade.Tradee,
			ItemName:  itemName,
			ItemID:    trade.ItemIDsTraded[i],
			HCValue:   trade.HCValuesTraded[i],
		})
	}

	// Add received items
	for i, itemName := range trade.ItemsReceived {
		tradeItems = append(tradeItems, TradeItem{
			Date:      time.Now(),
			Trader:    trade.Tradee,
			Recipient: trade.Trader,
			ItemName:  itemName,
			ItemID:    trade.ItemIDsReceived[i],
			HCValue:   trade.HCValuesReceived[i],
		})
	}

	jsonData, err := json.Marshal(tradeItems)
	if err != nil {
		return fmt.Errorf("failed to marshal trade data: %v", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/trade", APIBaseURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK response: %s", resp.Status)
	}

	return nil
}

func (m *Manager) playSound(filename string) {
	// Download the file
	resp, err := http.Get(AssetServerBaseURL + filename)
	if err != nil {
		log.Println("Error downloading sound file:", err)
		return
	}
	defer resp.Body.Close()

	// Create a temporary file
	tmpFile, err := ioutil.TempFile("", "sound-*.wav")
	if err != nil {
		log.Println("Error creating temporary file:", err)
		return
	}
	defer os.Remove(tmpFile.Name()) // Clean up

	// Copy the downloaded data to the temporary file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		log.Println("Error writing to temporary file:", err)
		return
	}
	tmpFile.Close()

	// Open the temporary file for playing
	f, err := os.Open(tmpFile.Name())
	if err != nil {
		log.Println("Error opening temporary sound file:", err)
		return
	}
	defer f.Close()

	streamer, format, err := wav.Decode(f)
	if err != nil {
		log.Println("Error decoding WAV file:", err)
		return
	}
	defer streamer.Close()

	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	if err != nil {
		log.Println("Error initializing speaker:", err)
		return
	}

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	<-done
}
