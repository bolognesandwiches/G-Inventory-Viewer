package ui

import (
	"fmt"
	"image/color"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/inventory"
	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/room"
)

type Manager struct {
	inventoryManager  *inventory.Manager
	roomManager       *room.Manager
	window            fyne.Window
	inventoryText     *widget.Entry
	summaryText       *widget.Entry
	iconContainer     *fyne.Container
	itemsEntry        *widget.Entry
	roomText          *widget.Entry
	roomSummaryText   *widget.Entry
	roomIconContainer *fyne.Container
	roomItemsEntry    *widget.Entry
	app               fyne.App
	mu                sync.Mutex // Mutex to handle concurrent access
}

func NewManager(invManager *inventory.Manager, roomManager *room.Manager) *Manager {
	return &Manager{
		inventoryManager: invManager,
		roomManager:      roomManager,
	}
}

func (m *Manager) Run() {
	m.mu.Lock()
	m.app = app.New()
	customTheme := &habboTheme{}
	m.app.Settings().SetTheme(customTheme)
	icon, _ := fyne.LoadResourceFromPath("./assets/scan_icon.png")
	m.app.SetIcon(icon)
	m.window = m.app.NewWindow("G-itemViewer")
	m.mu.Unlock()

	// Load header icons
	leftIcon := canvas.NewImageFromFile("./assets/left_icon.png")
	rightIcon := canvas.NewImageFromFile("./assets/right_icon.png")
	leftIcon.Resize(fyne.NewSize(100, 27))
	rightIcon.Resize(fyne.NewSize(100, 27))
	leftIcon.SetMinSize(fyne.NewSize(100, 27))
	rightIcon.SetMinSize(fyne.NewSize(100, 27))

	// Create title text
	titleText := canvas.NewText("G-itemViewer", color.White)
	titleText.Alignment = fyne.TextAlignCenter
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	// Create header container
	header := container.NewHBox(
		leftIcon,
		layout.NewSpacer(),
		titleText,
		layout.NewSpacer(),
		rightIcon,
	)

	inventoryTab := m.setupInventoryTab()
	roomSummaryTab := m.setupRoomSummaryTab()

	tabs := NewCustomTabContainer("Inventory", "Room")
	tabs.Refresh()

	content := container.NewMax()

	updateContent := func() {
		switch tabs.selected {
		case 0:
			content.Objects = []fyne.CanvasObject{inventoryTab}
			tabs.scanButton.OnTapped = func() {
				m.inventoryManager.ScanInventory()
			}
		case 1:
			content.Objects = []fyne.CanvasObject{roomSummaryTab}
			tabs.scanButton.OnTapped = func() {
				m.roomManager.ScanRoom()
			}
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

	m.inventoryManager.SetUpdateCallback(m.updateInventoryDisplay)
	m.roomManager.SetUpdateCallback(m.updateRoomDisplay)

	m.mu.Lock()
	m.window.Hide() // Initially hide the window
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

	roomSummaryContainer := m.createTitledContainer(m.roomSummaryText, "Room Summary")
	roomItemsContainer = m.createTitledContainer(roomItemsContainer, "Room Items")
	roomIdContainer := m.createTitledContainer(m.roomText, "Room Item IDs")

	return container.NewVBox(
		roomSummaryContainer,
		roomItemsContainer,
		roomIdContainer,
	)
}

func (m *Manager) createTitledContainer(content fyne.CanvasObject, title string) *fyne.Container {
	titleText := canvas.NewText(title, color.White)
	titleText.Alignment = fyne.TextAlignCenter
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.TextSize = 10
	titleText.TextStyle.Monospace = true
	return container.NewBorder(titleText, nil, nil, nil, content)
}

func (m *Manager) updateInventoryDisplay(items []inventory.EnrichedItem) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var displayText string
	iconURLs := make(map[string][]inventory.EnrichedItem)
	placeholderItems := make(map[string][]inventory.EnrichedItem)

	for _, item := range items {
		displayText += fmt.Sprintf("%s (%s)\n", item.Name, item.Class)
		if strings.HasSuffix(item.IconURL, "placeholder_icon.png") {
			key := item.Class
			if item.Props != "" {
				key += fmt.Sprintf(" (%s)", item.Props)
			}
			placeholderItems[key] = append(placeholderItems[key], item)
		} else {
			iconURLs[item.IconURL] = append(iconURLs[item.IconURL], item)
		}
	}

	m.inventoryText.SetText(displayText)
	m.summaryText.SetText(m.inventoryManager.GetInventorySummary())

	m.iconContainer.Objects = nil

	createButton := func(iconURL string, items []inventory.EnrichedItem) *widget.Button {
		btn := widget.NewButton("", func() {
			var itemIDsText strings.Builder
			itemIDsText.WriteString(fmt.Sprintf("Name: %s\n", items[0].Name))
			itemIDsText.WriteString(fmt.Sprintf("Count: %d\n", len(items)))
			itemIDsText.WriteString("IDs:\n")
			for _, item := range items {
				itemIDsText.WriteString(fmt.Sprintf("%d\n", item.ItemId))
			}
			m.inventoryText.SetText(itemIDsText.String())
		})

		btn.SetIcon(theme.AccountIcon())
		btn.Resize(fyne.NewSize(44, 44))

		go func() {
			resp, err := http.Get(iconURL)
			if err != nil {
				log.Printf("Error fetching icon: %v", err)
				return
			}
			defer resp.Body.Close()

			iconData, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Error reading icon data: %v", err)
				return
			}

			iconResource := fyne.NewStaticResource("icon", iconData)
			btn.SetIcon(iconResource)
			btn.Refresh()
		}()

		return btn
	}

	for iconURL, items := range iconURLs {
		button := createButton(iconURL, items)
		m.iconContainer.Add(button)
	}

	for _, items := range placeholderItems {
		button := createButton("https://images.habbo.com/dcr/hof_furni/56783/placeholder_icon.png", items)
		m.iconContainer.Add(button)
	}

	m.iconContainer.Refresh()
}

func (m *Manager) updateRoomDisplay(items []room.EnrichedItem) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var displayText string
	iconURLs := make(map[string][]room.EnrichedItem)
	placeholderItems := make(map[string][]room.EnrichedItem)

	for _, item := range items {
		displayText += fmt.Sprintf("%s (%s)\n", item.Name, item.Class)
		if strings.HasSuffix(item.IconURL, "placeholder_icon.png") {
			key := item.Class
			placeholderItems[key] = append(placeholderItems[key], item)
		} else {
			iconURLs[item.IconURL] = append(iconURLs[item.IconURL], item)
		}
	}

	m.roomText.SetText(displayText)
	m.roomSummaryText.SetText(m.roomManager.GetRoomSummary())

	m.roomIconContainer.Objects = nil

	createButton := func(iconURL string, items []room.EnrichedItem) *widget.Button {
		btn := widget.NewButton("", func() {
			var itemIDsText strings.Builder
			itemIDsText.WriteString(fmt.Sprintf("Name: %s\n", items[0].Name))
			itemIDsText.WriteString(fmt.Sprintf("Count: %d\n", len(items)))
			itemIDsText.WriteString("IDs:\n")
			for _, item := range items {
				if item.Type == "S" {
					itemIDsText.WriteString(fmt.Sprintf("%s (W:%d, H:%d, X:%d, Y:%d, Dir:%d)\n",
						item.Id, item.Width, item.Height, item.X, item.Y, item.Direction))
				} else {
					itemIDsText.WriteString(fmt.Sprintf("%s (Location: %s)\n", item.Id, item.Location))
				}
			}
			m.roomText.SetText(itemIDsText.String())
		})

		btn.SetIcon(theme.AccountIcon())
		btn.Resize(fyne.NewSize(44, 44))

		go func() {
			resp, err := http.Get(iconURL)
			if err != nil {
				log.Printf("Error fetching icon: %v", err)
				return
			}
			defer resp.Body.Close()

			iconData, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Error reading icon data: %v", err)
				return
			}

			iconResource := fyne.NewStaticResource("icon", iconData)
			btn.SetIcon(iconResource)
			btn.Refresh()
		}()

		return btn
	}

	for iconURL, items := range iconURLs {
		button := createButton(iconURL, items)
		m.roomIconContainer.Add(button)
	}

	for _, items := range placeholderItems {
		button := createButton("https://images.habbo.com/dcr/hof_furni/56783/placeholder_icon.png", items)
		m.roomIconContainer.Add(button)
	}

	m.roomIconContainer.Refresh()
}

type customScanButton struct {
	widget.Button
}

func newCustomScanButton(icon fyne.Resource, tapped func()) *customScanButton {
	button := &customScanButton{}
	button.ExtendBaseWidget(button)
	button.SetIcon(icon)
	button.OnTapped = tapped
	button.Importance = widget.LowImportance
	button.Alignment = widget.ButtonAlignCenter
	return button
}

func (b *customScanButton) CreateRenderer() fyne.WidgetRenderer {
	return &customButtonRenderer{button: &b.Button}
}

type customButtonRenderer struct {
	button *widget.Button
	icon   *canvas.Image
}

func (r *customButtonRenderer) Destroy() {}

func (r *customButtonRenderer) Layout(size fyne.Size) {
	if r.icon == nil {
		r.icon = canvas.NewImageFromResource(r.button.Icon)
	}
	r.icon.Resize(size)
}

func (r *customButtonRenderer) MinSize() fyne.Size {
	return fyne.NewSize(30, 30) // Adjust size as needed
}

func (r *customButtonRenderer) Objects() []fyne.CanvasObject {
	if r.icon == nil {
		r.icon = canvas.NewImageFromResource(r.button.Icon)
	}
	return []fyne.CanvasObject{r.icon}
}

func (r *customButtonRenderer) Refresh() {
	if r.icon != nil {
		r.icon.Refresh()
	}
}

type CustomTabContainer struct {
	widget.BaseWidget
	tabs       []*customTab
	selected   int
	content    fyne.CanvasObject
	OnChanged  func()
	scanButton *customScanButton
}

func NewCustomTabContainer(items ...string) *CustomTabContainer {
	c := &CustomTabContainer{
		selected: 0,
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

	c.scanButton = newCustomScanButton(loadScanIcon(), func() {})

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
	width += scanButtonSize.Width
	if scanButtonSize.Height > height {
		height = scanButtonSize.Height
	}
	return fyne.NewSize(width, height)
}

func (r *customTabContainerRenderer) Layout(size fyne.Size) {
	tabWidth := (size.Width - r.container.scanButton.MinSize().Width) / float32(len(r.container.tabs))
	for i, tab := range r.container.tabs {
		tab.Resize(fyne.NewSize(tabWidth, size.Height))
		tab.Move(fyne.NewPos(float32(i)*tabWidth, 0))
	}

	// Position scan button to the right of tabs
	scanButtonSize := r.container.scanButton.MinSize()
	r.container.scanButton.Resize(scanButtonSize)
	r.container.scanButton.Move(fyne.NewPos(size.Width-scanButtonSize.Width, (size.Height-scanButtonSize.Height)/2))
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
	r.outline.StrokeWidth = 3
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

func loadScanIcon() fyne.Resource {
	iconData, err := ioutil.ReadFile("assets/scan_icon.png")
	if err != nil {
		log.Printf("Failed to load scan icon: %v", err)
		return theme.SearchIcon()
	}
	return fyne.NewStaticResource("scan_icon", iconData)
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
	fontPaths := []string{
		"../" + filename,
		"assets/" + filename,
		filename,
	}

	for _, path := range fontPaths {
		data, err := ioutil.ReadFile(path)
		if err == nil {
			log.Printf("Successfully loaded font from: %s", path)
			return fyne.NewStaticResource(filename, data)
		}
		log.Printf("Failed to load font from %s: %v", path, err)
	}

	log.Printf("Failed to load %s, using default bold font", filename)
	return theme.DefaultTextBoldFont()
}
