package json

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/matoubidou/ksem/pkg/types"
	log "github.com/sirupsen/logrus"
)

// Handler implements the output.Handler interface for JSON output
type Handler struct {
	FilePath string
	Interval string
}

// NewHandler creates a new JSON output handler
func NewHandler(filePath string, interval string) *Handler {
	return &Handler{
		FilePath: filePath,
		Interval: interval,
	}
}

// Run starts the JSON output mode
func (h *Handler) Run(ctx context.Context, dataChan <-chan *types.KSEMData, errChan <-chan error) error {
	var lastData *types.KSEMData

	// Parse duration string
	duration, err := time.ParseDuration(h.Interval)
	if err != nil {
		log.Warnf("Invalid interval '%s', defaulting to 1s: %v", h.Interval, err)
		duration = time.Second
	}

	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errChan:
			return err
		case data := <-dataChan:
			// Buffer the latest data
			lastData = data
		case <-ticker.C:
			// Output the last received data on ticker
			if lastData != nil {
				if err := h.outputJSON(lastData); err != nil {
					log.Errorf("Error outputting JSON: %v", err)
				}
			}
		}
	}
}

func (h *Handler) outputJSON(data *types.KSEMData) error {
	// Create a more readable structure
	output := map[string]interface{}{
		"timestamp": data.Timestamp.Format("2006-01-02 15:04:05"),
		"power": map[string]interface{}{
			"solar_w":   data.PowerSolar,
			"battery_w": data.PowerBattery,
			"grid_w":    data.PowerGrid,
			"home_w":    data.PowerHome,
			"wallbox_w": data.PowerWallbox,
		},
		"battery": map[string]interface{}{
			"soc_percent": data.BatterySOC,
		},
		"grid": map[string]interface{}{
			"frequency_hz": data.GridFrequency,
		},
		"energy_totals_kwh": map[string]interface{}{
			"grid_purchase":     data.EnergyGridPurchase,
			"grid_feedin":       data.EnergyGridFeedIn,
			"solar_total":       data.EnergySolarTotal,
			"battery_charge":    data.EnergyBatteryCharge,
			"battery_discharge": data.EnergyBatteryDischarge,
			"wallbox":           data.EnergyWallbox,
		},
	}

	// Add phase power if available
	if data.ActivePowerTotal > 0 || data.ActivePowerL1 > 0 || data.ActivePowerL2 > 0 || data.ActivePowerL3 > 0 {
		output["phases_w"] = map[string]interface{}{
			"total": data.ActivePowerTotal,
			"l1":    data.ActivePowerL1,
			"l2":    data.ActivePowerL2,
			"l3":    data.ActivePowerL3,
		}
	}

	if h.FilePath != "" {
		// Pretty-print to file
		jsonData, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal data to JSON: %w", err)
		}
		if err := os.WriteFile(h.FilePath, jsonData, 0o644); err != nil {
			return fmt.Errorf("failed to write JSON file: %w", err)
		}
	} else {
		// Compact JSON for stdout
		compactData, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal data to JSON: %w", err)
		}
		fmt.Println(string(compactData))
	}

	return nil
}
