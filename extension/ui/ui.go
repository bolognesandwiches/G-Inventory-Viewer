package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/inventory"
)

type Manager struct {
	inventoryManager *inventory.Manager
	window           fyne.Window
	inventoryText    *widget.Entry
	summaryText      *widget.Label
	iconContainer    *fyne.Container
}

func NewManager(invManager *inventory.Manager) *Manager {
	return &Manager{
		inventoryManager: invManager,
	}
}

func (m *Manager) Run() {
	myApp := app.New()
	m.window = myApp.NewWindow("Habbo Inventory Viewer")

	m.inventoryText = widget.NewMultiLineEntry()
	m.inventoryText.SetText("Waiting for inventory scan...")
	m.inventoryText.Wrapping = fyne.TextWrapWord

	m.summaryText = widget.NewLabel("")
	m.iconContainer = container.NewGridWrap(fyne.NewSize(40, 40))

	content := container.NewVBox(
		widget.NewLabel("Inventory Summary:"),
		m.summaryText,
		widget.NewLabel("Icons:"),
		container.NewScroll(m.iconContainer),
		widget.NewLabel("Detailed Inventory:"),
		container.NewScroll(m.inventoryText),
	)

	m.window.SetContent(content)
	m.window.Resize(fyne.NewSize(800, 600))

	m.inventoryManager.SetUpdateCallback(m.updateInventoryDisplay)

	m.window.ShowAndRun()
}

func (m *Manager) updateInventoryDisplay(items []inventory.EnrichedItem) {
	var displayText string
	iconPaths := make(map[string]struct{})

	for _, item := range items {
		displayText += fmt.Sprintf("%s (%s): %s\n",
			item.FurniData.Name, item.Class, item.FurniData.Description)
		iconPaths[item.IconPath] = struct{}{}
	}

	m.inventoryText.SetText(displayText)
	m.summaryText.SetText(m.inventoryManager.GetInventorySummary())

	m.iconContainer.Objects = nil
	for iconPath := range iconPaths {
		icon := canvas.NewImageFromFile(iconPath)
		icon.FillMode = canvas.ImageFillOriginal
		icon.SetMinSize(fyne.NewSize(32, 32))
		m.iconContainer.Add(container.NewPadded(icon))
	}
	m.iconContainer.Refresh()
}
