package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mgoeppe/ksem/pkg/types"
	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

// Handler implements the output.Handler interface for SQLite output
type Handler struct {
	FilePath string
	Interval string
	db       *sql.DB
}

// NewHandler creates a new SQLite output handler
func NewHandler(filePath string, interval string) *Handler {
	return &Handler{
		FilePath: filePath,
		Interval: interval,
	}
}

// Run starts the SQLite output mode
func (h *Handler) Run(ctx context.Context, dataChan <-chan *types.KSEMData, errChan <-chan error) error {
	var lastData *types.KSEMData

	// Open database connection
	db, err := sql.Open("sqlite", h.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	h.db = db
	defer h.db.Close()

	// Create table if not exists
	if err := h.createTable(); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

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
			// Insert the last received data on ticker
			if lastData != nil {
				if err := h.insertData(lastData); err != nil {
					log.Errorf("Error inserting data into SQLite: %v", err)
				}
			}
		}
	}
}

func (h *Handler) createTable() error {
	createTableSQL := `CREATE TABLE IF NOT EXISTS ksem_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		power_solar REAL,
		power_battery REAL,
		power_grid REAL,
		power_home REAL,
		power_wallbox REAL,
		battery_soc INTEGER,
		grid_frequency REAL,
		energy_grid_purchase REAL,
		energy_grid_feedin REAL,
		energy_solar_total REAL,
		energy_battery_charge REAL,
		energy_battery_discharge REAL,
		energy_wallbox REAL,
		active_power_total REAL,
		active_power_l1 REAL,
		active_power_l2 REAL,
		active_power_l3 REAL
	);`

	_, err := h.db.Exec(createTableSQL)
	return err
}

func (h *Handler) insertData(data *types.KSEMData) error {
	insertSQL := `INSERT INTO ksem_data (
		timestamp, power_solar, power_battery, power_grid, power_home, power_wallbox,
		battery_soc, grid_frequency, energy_grid_purchase, energy_grid_feedin,
		energy_solar_total, energy_battery_charge, energy_battery_discharge,
		energy_wallbox, active_power_total, active_power_l1, active_power_l2, active_power_l3
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := h.db.Exec(insertSQL,
		data.Timestamp,
		data.PowerSolar,
		data.PowerBattery,
		data.PowerGrid,
		data.PowerHome,
		data.PowerWallbox,
		data.BatterySOC,
		data.GridFrequency,
		data.EnergyGridPurchase,
		data.EnergyGridFeedIn,
		data.EnergySolarTotal,
		data.EnergyBatteryCharge,
		data.EnergyBatteryDischarge,
		data.EnergyWallbox,
		data.ActivePowerTotal,
		data.ActivePowerL1,
		data.ActivePowerL2,
		data.ActivePowerL3,
	)
	if err != nil {
		return err
	}

	log.Debugf("Inserted data at %s into SQLite", data.Timestamp.Format("2006-01-02 15:04:05"))
	return nil
}
