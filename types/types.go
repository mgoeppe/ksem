package types

import "time"

// KSEMData represents the data structure for the KSEM meter
type KSEMData struct {
	Timestamp        time.Time `json:"timestamp"`
	ActivePowerTotal float64   `json:"active_power_total"` // 1-0:1.4.0*255 - Current total power
	ActivePowerL1    float64   `json:"active_power_l1"`    // 1-0:21.4.0*255
	ActivePowerL2    float64   `json:"active_power_l2"`    // 1-0:41.4.0*255
	ActivePowerL3    float64   `json:"active_power_l3"`    // 1-0:61.4.0*255
	GridFrequency    float64   `json:"grid_frequency"`     // 1-0:14.4.0*255

	// Instantaneous power flows (from sumvalues endpoint)
	PowerSolar   float64 `json:"power_solar"`   // Solar production (W, positive)
	PowerBattery float64 `json:"power_battery"` // Battery power (W, + charging, - discharging)
	PowerGrid    float64 `json:"power_grid"`    // Grid power (W, + importing, - exporting)
	PowerHome    float64 `json:"power_home"`    // Home consumption (W, positive)
	PowerWallbox float64 `json:"power_wallbox"` // Wallbox consumption (W, positive)
	BatterySOC   float64 `json:"battery_soc"`   // Battery state of charge (%)

	// Cumulative energy totals
	EnergyGridPurchase float64 `json:"energy_grid_purchase"` // Total purchased from grid
	EnergyGridFeedIn   float64 `json:"energy_grid_feedin"`   // Total fed into grid

	// Cumulative energy by source (if available)
	EnergySolarTotal       float64 `json:"energy_solar_total"`       // Total solar production
	EnergyBatteryCharge    float64 `json:"energy_battery_charge"`    // Total battery charged
	EnergyBatteryDischarge float64 `json:"energy_battery_discharge"` // Total battery discharged
	EnergyWallbox          float64 `json:"energy_wallbox"`           // Total wallbox consumption
}
