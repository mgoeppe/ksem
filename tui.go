package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type tuiModel struct {
	data       *KSEMData
	err        error
	quitting   bool
	lastUpdate time.Time
}

type dataUpdateMsg struct {
	data *KSEMData
}

type errMsg struct {
	err error
}

func initialModel() tuiModel {
	return tuiModel{}
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

func (m tuiModel) View() string {
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

func renderPowerFlow(data *KSEMData) string {
	var b strings.Builder

	// Visual flow diagram section header
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("246")).
		Bold(true).
		Render("⚡ Power Flow"))
	b.WriteString("\n\n")

	// Calculate battery SOC string for consistent width
	socStr := ""
	if data.BatterySOC > 0 {
		socStr = fmt.Sprintf("  [%.0f%%]", data.BatterySOC)
	}

	// Solar Production
	b.WriteString(labelStyle.Render("☀️  Solar:    "))
	b.WriteString(solarStyle.Render(fmt.Sprintf("%7.1f W", data.PowerSolar)))
	b.WriteString(strings.Repeat(" ", 15+len(socStr))) // Match longest line
	b.WriteString("\n")

	// Battery
	b.WriteString(labelStyle.Render("🔋 Battery:   "))
	if data.PowerBattery > 0 {
		b.WriteString(batteryChargingStyle.Render(fmt.Sprintf("%7.1f W ⬆ charging", data.PowerBattery)))
	} else if data.PowerBattery < 0 {
		b.WriteString(batteryDischargingStyle.Render(fmt.Sprintf("%7.1f W ⬇ discharging", data.PowerBattery)))
	} else {
		b.WriteString(valueStyle.Render(fmt.Sprintf("%7.1f W (idle)", data.PowerBattery)))
	}
	if socStr != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(socStr))
	}
	b.WriteString("\n")

	// Grid
	b.WriteString(labelStyle.Render("🔌 Grid:      "))
	if data.PowerGrid > 0 {
		b.WriteString(gridImportStyle.Render(fmt.Sprintf("%7.1f W ⬇ importing", data.PowerGrid)))
	} else if data.PowerGrid < 0 {
		b.WriteString(gridExportStyle.Render(fmt.Sprintf("%7.1f W ⬆ exporting", data.PowerGrid)))
	} else {
		b.WriteString(valueStyle.Render(fmt.Sprintf("%7.1f W", data.PowerGrid)))
		b.WriteString(strings.Repeat(" ", 13)) // Match status text length
	}
	b.WriteString(strings.Repeat(" ", len(socStr))) // Add SOC-width padding
	b.WriteString("\n")

	// Home Consumption
	b.WriteString(labelStyle.Render("🏠 Home:      "))
	b.WriteString(homeStyle.Render(fmt.Sprintf("%7.1f W", data.PowerHome)))
	b.WriteString(strings.Repeat(" ", 15+len(socStr))) // Match longest line
	b.WriteString("\n")

	// Wallbox (only if active)
	if data.PowerWallbox > 0 {
		b.WriteString(labelStyle.Render("🚗 Wallbox:   "))
		b.WriteString(valueStyle.Render(fmt.Sprintf("%7.1f W", data.PowerWallbox)))
		b.WriteString(strings.Repeat(" ", 15+len(socStr))) // Match longest line
		b.WriteString("\n")
	}

	return b.String()
}

func renderDetailedValues(data *KSEMData) string {
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

func sendDataUpdate(p *tea.Program, data *KSEMData) {
	p.Send(dataUpdateMsg{data: data})
}

func sendError(p *tea.Program, err error) {
	p.Send(errMsg{err: err})
}
