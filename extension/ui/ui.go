package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/bolognesandwiches/G-Inventory-Viewer/extension/inventory"
)

type Manager struct {
	inventoryManager *inventory.Manager
	window           fyne.Window
	inventoryText    *widget.Entry
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

	scrollContainer := container.NewScroll(m.inventoryText)
	refreshButton := widget.NewButton("Refresh Inventory", func() {
		go m.inventoryManager.ScanInventory(nil) // We'll need to modify this later
	})

	content := container.NewVBox(
		widget.NewLabel("Inventory:"),
		scrollContainer,
		refreshButton,
	)

	m.window.SetContent(container.NewPadded(content))
	m.window.Resize(fyne.NewSize(600, 400))

	m.inventoryManager.SetUpdateCallback(m.updateGUI)

	m.window.ShowAndRun()
}

func (m *Manager) updateGUI(text string) {
	if m.inventoryText != nil {
		m.inventoryText.SetText(text)
	}
}
