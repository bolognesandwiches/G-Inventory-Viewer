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
	scrollContainer  *container.Scroll
}

func NewManager(invManager *inventory.Manager) *Manager {
	return &Manager{
		inventoryManager: invManager,
	}
}

func (m *Manager) Run() {
	myApp := app.New()
	m.window = myApp.NewWindow("Habbo Inventory Viewer")

	content := container.NewVBox(
		widget.NewLabel("Inventory:"),
	)

	m.scrollContainer = container.NewScroll(content)
	refreshButton := widget.NewButton("Refresh Inventory", func() {
		go m.inventoryManager.ScanInventory(nil)
	})

	layout := container.NewBorder(refreshButton, nil, nil, nil, m.scrollContainer)
	m.window.SetContent(layout)
	m.window.Resize(fyne.NewSize(600, 400))

	m.inventoryManager.SetUpdateCallback(m.updateInventoryDisplay)

	m.window.ShowAndRun()
}

func (m *Manager) updateInventoryDisplay(items []inventory.EnrichedItem) {
	content := container.NewVBox()

	for _, item := range items {
		itemBox := container.NewHBox()

		// Load and display icon
		icon := canvas.NewImageFromFile(item.IconPath)
		icon.FillMode = canvas.ImageFillOriginal
		icon.SetMinSize(fyne.NewSize(32, 32))
		itemBox.Add(icon)

		// Display item info
		info := widget.NewLabel(fmt.Sprintf("%s: %s\nDescription: %s",
			item.FurniData.Name, item.Class, item.FurniData.Description))
		itemBox.Add(info)

		content.Add(itemBox)
	}

	m.scrollContainer.Content = content
	m.scrollContainer.Refresh()
}
