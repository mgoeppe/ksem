package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matoubidou/ksem/types"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			Background(lipgloss.Color("235")).
			Padding(0, 1).
			MarginBottom(1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("246"))

	valueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255"))

	solarStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("220")) // Yellow/gold for solar

	batteryChargingStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("46")) // Green for charging

	batteryDischargingStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("196")) // Red for discharging

	gridImportStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")) // Red for importing

	gridExportStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("46")) // Green for exporting

	homeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("69")) // Blue for home

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginTop(1)
)

type model struct {
	data       *types.KSEMData
	err        error
	quitting   bool
	lastUpdate time.Time
	dataChan   <-chan *types.KSEMData
	errChan    <-chan error
}

type dataUpdateMsg struct {
	data *types.KSEMData
}

type errMsg struct {
	err error
}

func initialModel(dataChan <-chan *types.KSEMData, errChan <-chan error) model {
	return model{
		dataChan: dataChan,
		errChan:  errChan,
	}
}

// waitForData waits for data from channels
func waitForData(dataChan <-chan *types.KSEMData, errChan <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case data := <-dataChan:
			return dataUpdateMsg{data: data}
		case err := <-errChan:
			return errMsg{err: err}
		}
	}
}

func (m model) Init() tea.Cmd {
	// Start listening to channels immediately
	return waitForData(m.dataChan, m.errChan)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}

	case dataUpdateMsg:
		m.data = msg.data
		m.lastUpdate = time.Now()
		// Immediately wait for next message (event-driven)
		return m, waitForData(m.dataChan, m.errChan)

	case errMsg:
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "Shutting down...\n"
	}

	var b strings.Builder

	// Title
	title := titleStyle.Render("⚡ KSEM Energy Monitor")
	b.WriteString(title + "\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
	} else if m.data == nil {
		b.WriteString("Waiting for data...\n")
	} else {
		// Timestamp
		timestamp := m.data.Timestamp.Format("2006-01-02 15:04:05")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(timestamp))
		b.WriteString("\n\n")

		// Power Flow Visualization
		b.WriteString(renderPowerFlow(m.data))

		// Detailed values
		details := renderDetailedValues(m.data)
		if details != "" {
			b.WriteString("\n\n")
			b.WriteString(details)
		}
	}

	content := boxStyle.Render(b.String())
	help := helpStyle.Render("Press 'q' or Ctrl+C to quit")

	return content + "\n" + help
}

func renderPowerFlow(data *types.KSEMData) string {
	var rows []string

	// Section header
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("246")).
		Bold(true).
		Render("⚡ Power Flow")

	rows = append(rows, header)
	rows = append(rows, "") // blank line

	// Create fixed-width columns for alignment
	col1Width := 15 // emoji + label (increased for wider emojis)
	col2Width := 12 // value
	col3Width := 17 // extra text

	createRow := func(emoji, label, valueStr, extraStr string) string {
		// Column 1: emoji + label (use plain string formatting, not lipgloss Width)
		labelText := fmt.Sprintf("%s %-10s", emoji, label)
		col1 := lipgloss.NewStyle().
			Width(col1Width).
			Render(labelStyle.Render(labelText))

		// Column 2: value (right-aligned)
		col2 := lipgloss.NewStyle().
			Width(col2Width).
			Align(lipgloss.Right).
			Render(valueStr)

		// Column 3: extra text
		col3 := lipgloss.NewStyle().
			Width(col3Width).
			Render(extraStr)

		return lipgloss.JoinHorizontal(lipgloss.Left, col1, col2, col3)
	}

	// Solar Production
	solarValue := solarStyle.Render(fmt.Sprintf("%.1f W", data.PowerSolar))
	rows = append(rows, createRow("💡", "Solar:", solarValue, ""))

	// Battery
	var batteryValue, batteryExtra string
	if data.PowerBattery > 0 {
		batteryValue = batteryChargingStyle.Render(fmt.Sprintf("%.1f W", data.PowerBattery))
		batteryExtra = batteryChargingStyle.Render("⬆ charging")
	} else if data.PowerBattery < 0 {
		batteryValue = batteryDischargingStyle.Render(fmt.Sprintf("%.1f W", data.PowerBattery))
		batteryExtra = batteryDischargingStyle.Render("⬇ discharging")
	} else {
		batteryValue = valueStyle.Render(fmt.Sprintf("%.1f W", data.PowerBattery))
		batteryExtra = valueStyle.Render("(idle)")
	}
	if data.BatterySOC > 0 {
		batteryExtra += lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(fmt.Sprintf(" [%.0f%%]", data.BatterySOC))
	}
	rows = append(rows, createRow("🔋", "Battery:", batteryValue, batteryExtra))

	// Grid
	var gridValue, gridExtra string
	if data.PowerGrid > 0 {
		gridValue = gridImportStyle.Render(fmt.Sprintf("%.1f W", data.PowerGrid))
		gridExtra = gridImportStyle.Render("⬇ importing")
	} else if data.PowerGrid < 0 {
		gridValue = gridExportStyle.Render(fmt.Sprintf("%.1f W", data.PowerGrid))
		gridExtra = gridExportStyle.Render("⬆ exporting")
	} else {
		gridValue = valueStyle.Render(fmt.Sprintf("%.1f W", data.PowerGrid))
		gridExtra = ""
	}
	rows = append(rows, createRow("🔌", "Grid:", gridValue, gridExtra))

	// Home Consumption
	homeValue := homeStyle.Render(fmt.Sprintf("%.1f W", data.PowerHome))
	rows = append(rows, createRow("🏠", "Home:", homeValue, ""))

	// Wallbox (only if active)
	if data.PowerWallbox > 0 {
		wallboxValue := valueStyle.Render(fmt.Sprintf("%.1f W", data.PowerWallbox))
		rows = append(rows, createRow("🚗", "Wallbox:", wallboxValue, ""))
	}

	return strings.Join(rows, "\n")
}

func renderDetailedValues(data *types.KSEMData) string {
	var b strings.Builder

	// Only show cumulative totals if available
	if data.EnergyGridPurchase > 0 || data.EnergyGridFeedIn > 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Underline(true).
			Render("Cumulative Energy Totals"))
		b.WriteString("\n")

		b.WriteString(labelStyle.Render("Grid Purchase:"))
		b.WriteString(valueStyle.Render(fmt.Sprintf("%.2f kWh", data.EnergyGridPurchase)))
		b.WriteString("\n")

		b.WriteString(labelStyle.Render("Grid Feed-in:"))
		b.WriteString(valueStyle.Render(fmt.Sprintf("%.2f kWh", data.EnergyGridFeedIn)))
		b.WriteString("\n")

		if data.EnergySolarTotal > 0 {
			b.WriteString(labelStyle.Render("Solar Total:"))
			b.WriteString(valueStyle.Render(fmt.Sprintf("%.2f kWh", data.EnergySolarTotal)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// Handler implements the output.Handler interface for TUI output
type Handler struct{}

// NewHandler creates a new TUI output handler
func NewHandler() *Handler {
	return &Handler{}
}

// Run starts the TUI with the given data and error channels
func (h *Handler) Run(ctx context.Context, dataChan <-chan *types.KSEMData, errChan <-chan error) error {
	m := initialModel(dataChan, errChan)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
