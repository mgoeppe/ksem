package obis

import "fmt"

// OBIS code definition with metadata
type Code struct {
	Hex         uint64
	Description string
	Unit        Unit
	ScaleFactor float64 // Factor to convert raw value to unit
}

// Unit types for OBIS codes
type Unit string

const (
	Watt         Unit = "W"
	WattHour     Unit = "Wh"
	KiloWatt     Unit = "kW"
	KiloWattHour Unit = "kWh"
	Hertz        Unit = "Hz"
	Ampere       Unit = "A"
	Volt         Unit = "V"
)

// Standard OBIS codes for KSEM meter
// Format: 0x1 00 AA BB CC FF where BB=04 for power (W), BB=08 for energy (Wh)
var (
	// Instantaneous power measurements
	ActivePowerTotal = Code{
		Hex:         0x100010400FF,
		Description: "Total active power (1-0:1.4.0*255)",
		Unit:        Watt,
		ScaleFactor: 0.001, // mW to W
	}
	ActivePowerL1 = Code{
		Hex:         0x100150400FF,
		Description: "L1 active power (1-0:21.4.0*255)",
		Unit:        Watt,
		ScaleFactor: 0.001,
	}
	ActivePowerL2 = Code{
		Hex:         0x100290400FF,
		Description: "L2 active power (1-0:41.4.0*255)",
		Unit:        Watt,
		ScaleFactor: 0.001,
	}
	ActivePowerL3 = Code{
		Hex:         0x1003D0400FF,
		Description: "L3 active power (1-0:61.4.0*255)",
		Unit:        Watt,
		ScaleFactor: 0.001,
	}
	GridFrequency = Code{
		Hex:         0x1000E0400FF,
		Description: "Grid frequency (1-0:14.4.0*255)",
		Unit:        Hertz,
		ScaleFactor: 0.001, // mHz to Hz
	}

	// Cumulative energy totals
	EnergyGridPurchase = Code{
		Hex:         0x100010800FF,
		Description: "Grid purchase (1-0:1.8.0*255)",
		Unit:        KiloWattHour,
		ScaleFactor: 0.000001, // mWh to kWh
	}
	EnergyGridFeedIn = Code{
		Hex:         0x100020800FF,
		Description: "Grid feed-in (1-0:2.8.0*255)",
		Unit:        KiloWattHour,
		ScaleFactor: 0.000001,
	}
	EnergySolarTotal = Code{
		Hex:         0x100460800FF,
		Description: "Total solar production",
		Unit:        KiloWattHour,
		ScaleFactor: 0.000001,
	}
	EnergyBatteryCharge = Code{
		Hex:         0x1003E0800FF,
		Description: "Total battery charge",
		Unit:        KiloWattHour,
		ScaleFactor: 0.000001,
	}
	EnergyBatteryDischarge = Code{
		Hex:         0x1003D0800FF,
		Description: "Total battery discharge",
		Unit:        KiloWattHour,
		ScaleFactor: 0.000001,
	}
	EnergyWallbox = Code{
		Hex:         0x100450800FF,
		Description: "Total wallbox consumption",
		Unit:        KiloWattHour,
		ScaleFactor: 0.000001,
	}
)

// Convert converts a raw OBIS value to the appropriate unit
func (c Code) Convert(rawValue uint64) float64 {
	return float64(rawValue) * c.ScaleFactor
}

// String returns a formatted string with value and unit
func (c Code) Format(rawValue uint64) string {
	return fmt.Sprintf("%.2f %s", c.Convert(rawValue), c.Unit)
}

// Registry for looking up codes by hex value
var Registry = map[uint64]Code{
	ActivePowerTotal.Hex:       ActivePowerTotal,
	ActivePowerL1.Hex:          ActivePowerL1,
	ActivePowerL2.Hex:          ActivePowerL2,
	ActivePowerL3.Hex:          ActivePowerL3,
	GridFrequency.Hex:          GridFrequency,
	EnergyGridPurchase.Hex:     EnergyGridPurchase,
	EnergyGridFeedIn.Hex:       EnergyGridFeedIn,
	EnergySolarTotal.Hex:       EnergySolarTotal,
	EnergyBatteryCharge.Hex:    EnergyBatteryCharge,
	EnergyBatteryDischarge.Hex: EnergyBatteryDischarge,
	EnergyWallbox.Hex:          EnergyWallbox,
}

// Lookup finds an OBIS code by its hex value
func Lookup(hex uint64) (Code, bool) {
	code, ok := Registry[hex]
	return code, ok
}

// ParsedData holds all parsed OBIS and flex values
type ParsedData struct {
	// OBIS values
	ActivePowerTotal       float64
	ActivePowerL1          float64
	ActivePowerL2          float64
	ActivePowerL3          float64
	GridFrequency          float64
	EnergyGridPurchase     float64
	EnergyGridFeedIn       float64
	EnergySolarTotal       float64
	EnergyBatteryCharge    float64
	EnergyBatteryDischarge float64
	EnergyWallbox          float64

	// Flex values (instantaneous power flows)
	PowerSolar   float64
	PowerBattery float64
	PowerGrid    float64
	PowerHome    float64
	PowerWallbox float64
	BatterySOC   float64
}

// ParseValues extracts and converts all OBIS and flex values
func ParseValues(obisValues map[uint64]uint64, flexValues map[string]int64, debug bool) *ParsedData {
	result := &ParsedData{}

	// Parse OBIS values
	if obisValues != nil {
		if val, ok := obisValues[ActivePowerTotal.Hex]; ok {
			result.ActivePowerTotal = ActivePowerTotal.Convert(val)
		}
		if val, ok := obisValues[ActivePowerL1.Hex]; ok {
			result.ActivePowerL1 = ActivePowerL1.Convert(val)
		}
		if val, ok := obisValues[ActivePowerL2.Hex]; ok {
			result.ActivePowerL2 = ActivePowerL2.Convert(val)
		}
		if val, ok := obisValues[ActivePowerL3.Hex]; ok {
			result.ActivePowerL3 = ActivePowerL3.Convert(val)
		}
		if val, ok := obisValues[GridFrequency.Hex]; ok {
			result.GridFrequency = GridFrequency.Convert(val)
		}

		// Cumulative energy totals
		if val, ok := obisValues[EnergyGridPurchase.Hex]; ok {
			result.EnergyGridPurchase = EnergyGridPurchase.Convert(val)
		}
		if val, ok := obisValues[EnergyGridFeedIn.Hex]; ok {
			result.EnergyGridFeedIn = EnergyGridFeedIn.Convert(val)
		}
		if val, ok := obisValues[EnergySolarTotal.Hex]; ok {
			result.EnergySolarTotal = EnergySolarTotal.Convert(val)
		}
		if val, ok := obisValues[EnergyBatteryCharge.Hex]; ok {
			result.EnergyBatteryCharge = EnergyBatteryCharge.Convert(val)
		}
		if val, ok := obisValues[EnergyBatteryDischarge.Hex]; ok {
			result.EnergyBatteryDischarge = EnergyBatteryDischarge.Convert(val)
		}
		if val, ok := obisValues[EnergyWallbox.Hex]; ok {
			result.EnergyWallbox = EnergyWallbox.Convert(val)
		}
	}

	// Parse flex values (instantaneous power flows)
	if flexValues != nil {
		// Solar power (pvPowerTotal or pvPowerACSum) - negative means producing
		if val, ok := flexValues["pvPowerTotal"]; ok {
			result.PowerSolar = -float64(val) / 1000.0 // Invert sign and mW to W
		} else if val, ok := flexValues["pvPowerACSum"]; ok {
			result.PowerSolar = -float64(val) / 1000.0
		}

		// Battery power - positive = charging, negative = discharging
		if val, ok := flexValues["batteryPowerTotal"]; ok {
			result.PowerBattery = float64(val) / 1000.0 // mW to W
		}

		// Grid power - positive = importing, negative = exporting
		if val, ok := flexValues["gridPowerTotal"]; ok {
			result.PowerGrid = float64(val) / 1000.0 // mW to W
		}

		// Home consumption
		if val, ok := flexValues["housePowerTotal"]; ok {
			result.PowerHome = float64(val) / 1000.0 // mW to W
		}

		// Wallbox consumption
		if val, ok := flexValues["wallboxPowerTotal"]; ok {
			result.PowerWallbox = float64(val) / 1000.0 // mW to W
		}

		// Battery state of charge (%)
		if val, ok := flexValues["systemStateOfCharge"]; ok {
			result.BatterySOC = float64(val) // Already in %
		}
	}

	return result
}
