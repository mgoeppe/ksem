package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	pb "github.com/matoubidou/ksem/proto"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

// Config holds all configuration settings
type Config struct {
	Meter struct {
		Host     string `yaml:"host"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"meter"`
	OAuth2 struct {
		ClientID     string `yaml:"client_id"`
		ClientSecret string `yaml:"client_secret"`
	} `yaml:"oauth2"`
	Scraping struct {
		Interval string `yaml:"interval"`
		ConfigID string `yaml:"config_id"` // e.g., "smart-meter"
	} `yaml:"scraping"`
	Output struct {
		Format   string `yaml:"format"`
		FilePath string `yaml:"file_path"`
	} `yaml:"output"`
	Debug bool `yaml:"debug"`
}

// KSEMData represents the data structure for the KSEM meter
type KSEMData struct {
	Timestamp          time.Time `json:"timestamp"`
	ActivePowerTotal   float64   `json:"active_power_total"`   // 1-0:1.4.0*255 - Current total power
	ActivePowerL1      float64   `json:"active_power_l1"`      // 1-0:21.4.0*255
	ActivePowerL2      float64   `json:"active_power_l2"`      // 1-0:41.4.0*255
	ActivePowerL3      float64   `json:"active_power_l3"`      // 1-0:61.4.0*255
	GridFrequency      float64   `json:"grid_frequency"`       // 1-0:14.4.0*255
	
	// Instantaneous power flows (from sumvalues endpoint)
	PowerSolar         float64   `json:"power_solar"`         // Solar production (W, positive)
	PowerBattery       float64   `json:"power_battery"`       // Battery power (W, + charging, - discharging)
	PowerGrid          float64   `json:"power_grid"`          // Grid power (W, + importing, - exporting)
	PowerHome          float64   `json:"power_home"`          // Home consumption (W, positive)
	PowerWallbox       float64   `json:"power_wallbox"`       // Wallbox consumption (W, positive)
	BatterySOC         float64   `json:"battery_soc"`         // Battery state of charge (%)
	
	// Cumulative energy totals
	EnergyGridPurchase float64   `json:"energy_grid_purchase"` // Total purchased from grid
	EnergyGridFeedIn   float64   `json:"energy_grid_feedin"`   // Total fed into grid
	
	// Cumulative energy by source (if available)
	EnergySolarTotal   float64   `json:"energy_solar_total"`   // Total solar production
	EnergyBatteryCharge float64  `json:"energy_battery_charge"` // Total battery charged
	EnergyBatteryDischarge float64 `json:"energy_battery_discharge"` // Total battery discharged  
	EnergyWallbox      float64   `json:"energy_wallbox"`       // Total wallbox consumption
}

// OBIS code constants (encoded as seen in the WebSocket data)
// Format: 0x1 00 AA BB CC FF where BB=04 for power (W), BB=08 for energy (Wh)
const (
	// Instantaneous power measurements (BB=04)
	OBIS_ACTIVE_POWER_TOTAL   uint64 = 0x100010400FF  // 1-0:1.4.0*255 - Total active power
	OBIS_ACTIVE_POWER_L1       uint64 = 0x100150400FF  // 1-0:21.4.0*255 - L1 active power
	OBIS_ACTIVE_POWER_L2       uint64 = 0x100290400FF  // 1-0:41.4.0*255 - L2 active power
	OBIS_ACTIVE_POWER_L3       uint64 = 0x1003D0400FF  // 1-0:61.4.0*255 - L3 active power
	OBIS_GRID_FREQUENCY        uint64 = 0x1000E0400FF  // 1-0:14.4.0*255 - Grid frequency
	
	// Cumulative energy totals (BB=08) - these match the UI bottom display
	OBIS_ENERGY_GRID_PURCHASE  uint64 = 0x100010800FF  // 1-0:1.8.0*255 - Grid purchase ("Purchase" in UI)
	OBIS_ENERGY_GRID_FEEDIN    uint64 = 0x100020800FF  // 1-0:2.8.0*255 - Grid feed-in ("Feed-in" in UI)
	
	// Additional cumulative energy totals by source
	OBIS_ENERGY_SOLAR_TOTAL    uint64 = 0x100460800FF  // Total solar production
	OBIS_ENERGY_BATTERY_CHARGE uint64 = 0x1003E0800FF  // Total battery charge
	OBIS_ENERGY_BATTERY_DISCHARGE uint64 = 0x1003D0800FF // Total battery discharge
	OBIS_ENERGY_WALLBOX        uint64 = 0x100450800FF  // Total wallbox consumption
)

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.Meter.Username == "" {
		config.Meter.Username = "admin"
	}
	if config.OAuth2.ClientID == "" {
		config.OAuth2.ClientID = "emos"
	}
	if config.OAuth2.ClientSecret == "" {
		config.OAuth2.ClientSecret = "56951025"
	}
	if config.Scraping.Interval == "" {
		config.Scraping.Interval = "10s"
	}
	if config.Scraping.ConfigID == "" {
		config.Scraping.ConfigID = "smart-meter"
	}
	if config.Output.Format == "" {
		config.Output.Format = "console"
	}

	return &config, nil
}

func authenticate(ctx context.Context, config *Config) (*oauth2.Token, error) {
	tokenURL := fmt.Sprintf("http://%s/api/web-login/token", config.Meter.Host)

	oauth2Config := oauth2.Config{
		ClientID:     config.OAuth2.ClientID,
		ClientSecret: config.OAuth2.ClientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: tokenURL,
		},
	}

	// Username is always "admin" for KSEM
	username := "admin"
	if config.Meter.Username != "" {
		username = config.Meter.Username
	}

	token, err := oauth2Config.PasswordCredentialsToken(ctx, username, config.Meter.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain token: %w", err)
	}

	if config.Debug {
		log.Printf("Authentication successful, token type: %s", token.TokenType)
	}

	return token, nil
}

func connectWebSocket(config *Config, token *oauth2.Token) (*websocket.Conn, error) {
	// Build WebSocket URL
	// Most endpoints use /values/ path
	wsURL := url.URL{
		Scheme: "ws",
		Host:   config.Meter.Host,
		Path:   fmt.Sprintf("/api/data-transfer/ws/protobuf/gdr/local/values/%s", config.Scraping.ConfigID),
	}

	if config.Debug {
		log.Printf("Connecting to WebSocket: %s", wsURL.String())
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	// Send Bearer token as first message (authentication)
	authMsg := fmt.Sprintf("%s %s", token.TokenType, token.AccessToken)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(authMsg)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send auth token: %w", err)
	}

	if config.Debug {
		log.Println("WebSocket connected and authenticated")
	}

	return conn, nil
}

func parseProtobufMessage(data []byte, config *Config) (*KSEMData, error) {
	if config.Debug {
		log.Printf("Received %d bytes of protobuf data", len(data))
	}

	result := &KSEMData{
		Timestamp: time.Now(),
	}

	// Parse the GDRs message using generated protobuf code
	gdrs := &pb.GDRs{}
	if err := proto.Unmarshal(data, gdrs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GDRs: %w", err)
	}

	// Extract values from all GDRs
	var allValues map[uint64]uint64
	var allFlexValues map[string]int64

	for _, gdr := range gdrs.GDRs {
		// Merge OBIS values
		if len(gdr.Values) > 0 {
			if allValues == nil {
				allValues = make(map[uint64]uint64)
			}
			for k, v := range gdr.Values {
				allValues[k] = v
			}
		}

		// Merge flex values
		if len(gdr.FlexValues) > 0 {
			if allFlexValues == nil {
				allFlexValues = make(map[string]int64)
			}
			for k, v := range gdr.FlexValues {
				allFlexValues[k] = v.IntValue
			}
		}
	}

	// Log flex values for debugging
	if config.Debug && len(allFlexValues) > 0 {
		log.Printf("Parsed %d flex values from protobuf message", len(allFlexValues))
		for key, value := range allFlexValues {
			log.Printf("  Flex[%s] = %d (%.2f W)", key, value, float64(value)/1000.0)
		}
	}

	// Extract OBIS values and convert units (mW -> W, mHz -> Hz, mWh -> kWh)
	if allValues != nil {
		if config.Debug {
			log.Printf("Parsed %d OBIS values from protobuf message", len(allValues))
			// Log ALL OBIS codes with their values to identify energy flows
			for obisCode, value := range allValues {
				log.Printf("  OBIS %d (0x%X) = %d (%.2f W or %.3f kWh)", obisCode, obisCode, value, float64(value)/1000.0, float64(value)/1000000.0)
			}
		}

		if val, ok := allValues[OBIS_ACTIVE_POWER_TOTAL]; ok {
			result.ActivePowerTotal = float64(val) / 1000.0 // mW to W
		}
		if val, ok := allValues[OBIS_ACTIVE_POWER_L1]; ok {
			result.ActivePowerL1 = float64(val) / 1000.0 // mW to W
		}
		if val, ok := allValues[OBIS_ACTIVE_POWER_L2]; ok {
			result.ActivePowerL2 = float64(val) / 1000.0 // mW to W
		}
		if val, ok := allValues[OBIS_ACTIVE_POWER_L3]; ok {
			result.ActivePowerL3 = float64(val) / 1000.0 // mW to W
		}
		if val, ok := allValues[OBIS_GRID_FREQUENCY]; ok {
			result.GridFrequency = float64(val) / 1000.0 // mHz to Hz
		}
		
		// Cumulative energy totals
		if val, ok := allValues[OBIS_ENERGY_GRID_PURCHASE]; ok {
			result.EnergyGridPurchase = float64(val) / 1000000.0 // mWh to kWh
		}
		if val, ok := allValues[OBIS_ENERGY_GRID_FEEDIN]; ok {
			result.EnergyGridFeedIn = float64(val) / 1000000.0 // mWh to kWh
		}
		if val, ok := allValues[OBIS_ENERGY_SOLAR_TOTAL]; ok {
			result.EnergySolarTotal = float64(val) / 1000000.0 // mWh to kWh
		}
		if val, ok := allValues[OBIS_ENERGY_BATTERY_CHARGE]; ok {
			result.EnergyBatteryCharge = float64(val) / 1000000.0 // mWh to kWh
		}
		if val, ok := allValues[OBIS_ENERGY_BATTERY_DISCHARGE]; ok {
			result.EnergyBatteryDischarge = float64(val) / 1000000.0 // mWh to kWh
		}
		if val, ok := allValues[OBIS_ENERGY_WALLBOX]; ok {
			result.EnergyWallbox = float64(val) / 1000000.0 // mWh to kWh
		}
		
		// Note: L1 and L2 phase powers may not be available from all KSEM configurations
		// The device may only provide total power and L3 phase power
	}

	// Extract flex values (from sumvalues endpoint)
	if allFlexValues != nil {
		// Solar power (pvPowerTotal or pvPowerACSum) - negative means producing
		if val, ok := allFlexValues["pvPowerTotal"]; ok {
			result.PowerSolar = -float64(val) / 1000.0 // Invert sign and mW to W
		} else if val, ok := allFlexValues["pvPowerACSum"]; ok {
			result.PowerSolar = -float64(val) / 1000.0
		}

		// Battery power - positive = charging, negative = discharging
		if val, ok := allFlexValues["batteryPowerTotal"]; ok {
			result.PowerBattery = float64(val) / 1000.0 // mW to W
		}

		// Grid power - positive = importing, negative = exporting
		if val, ok := allFlexValues["gridPowerTotal"]; ok {
			result.PowerGrid = float64(val) / 1000.0 // mW to W
		}

		// Home consumption
		if val, ok := allFlexValues["housePowerTotal"]; ok {
			result.PowerHome = float64(val) / 1000.0 // mW to W
		}

		// Wallbox consumption
		if val, ok := allFlexValues["wallboxPowerTotal"]; ok {
			result.PowerWallbox = float64(val) / 1000.0 // mW to W
		}

		// Battery state of charge (%)
		if val, ok := allFlexValues["systemStateOfCharge"]; ok {
			result.BatterySOC = float64(val) // Already in %
		}
	}

	return result, nil
}

func outputData(config *Config, data *KSEMData) error {
	switch config.Output.Format {
	case "json":
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal data to JSON: %w", err)
		}

		if config.Output.FilePath != "" {
			if err := os.WriteFile(config.Output.FilePath, jsonData, 0644); err != nil {
				return fmt.Errorf("failed to write JSON file: %w", err)
			}
		} else {
			fmt.Println(string(jsonData))
		}

	case "console":
		fmt.Printf("\n=== KSEM Data at %s ===\n", data.Timestamp.Format("2006-01-02 15:04:05"))
		
		// Show instantaneous power flows if available (from sumvalues endpoint)
		if data.PowerSolar > 0 || data.PowerBattery != 0 || data.PowerGrid != 0 || data.PowerHome > 0 {
			fmt.Printf("\n--- Instantaneous Power Flows ---\n")
			fmt.Printf("Solar Production:   %.2f W\n", data.PowerSolar)
			
			// Battery: show charging/discharging with direction
			if data.PowerBattery > 0 {
				fmt.Printf("Battery:            +%.2f W (charging)\n", data.PowerBattery)
			} else if data.PowerBattery < 0 {
				fmt.Printf("Battery:            %.2f W (discharging)\n", data.PowerBattery)
			} else {
				fmt.Printf("Battery:            %.2f W (idle)\n", data.PowerBattery)
			}
			
			if data.BatterySOC > 0 {
				fmt.Printf("Battery SOC:        %.0f%%\n", data.BatterySOC)
			}
			
			// Grid: show importing/exporting with direction
			if data.PowerGrid > 0 {
				fmt.Printf("Grid:               +%.2f W (importing)\n", data.PowerGrid)
			} else if data.PowerGrid < 0 {
				fmt.Printf("Grid:               %.2f W (exporting)\n", data.PowerGrid)
			} else {
				fmt.Printf("Grid:               %.2f W\n", data.PowerGrid)
			}
			
			fmt.Printf("Home Consumption:   %.2f W\n", data.PowerHome)
			fmt.Printf("Wallbox:            %.2f W\n", data.PowerWallbox)
		}
		
		// Show phase power if available (from smart-meter endpoint)
		if data.ActivePowerTotal > 0 || data.ActivePowerL1 > 0 || data.ActivePowerL2 > 0 || data.ActivePowerL3 > 0 {
			fmt.Printf("\n--- Phase Power ---\n")
			fmt.Printf("Active Power Total: %.2f W\n", data.ActivePowerTotal)
			fmt.Printf("Active Power L1:    %.2f W\n", data.ActivePowerL1)
			fmt.Printf("Active Power L2:    %.2f W\n", data.ActivePowerL2)
			fmt.Printf("Active Power L3:    %.2f W\n", data.ActivePowerL3)
		}
		
		if data.GridFrequency > 0 {
			fmt.Printf("Grid Frequency:     %.2f Hz\n", data.GridFrequency)
		}
		
		// Show cumulative totals if available
		if data.EnergyGridPurchase > 0 || data.EnergyGridFeedIn > 0 {
			fmt.Printf("\n--- Cumulative Energy Totals ---\n")
			fmt.Printf("Grid Purchase:      %.3f kWh\n", data.EnergyGridPurchase)
			fmt.Printf("Grid Feed-in:       %.3f kWh\n", data.EnergyGridFeedIn)
			fmt.Printf("Solar Production:   %.3f kWh\n", data.EnergySolarTotal)
			fmt.Printf("Battery Charged:    %.3f kWh\n", data.EnergyBatteryCharge)
			fmt.Printf("Battery Discharged: %.3f kWh\n", data.EnergyBatteryDischarge)
			fmt.Printf("Wallbox:            %.3f kWh\n", data.EnergyWallbox)
		}

	default:
		return fmt.Errorf("unknown output format: %s", config.Output.Format)
	}

	return nil
}

func main() {
	// Load configuration
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if config.Debug {
		log.Println("Debug mode enabled")
	}

	// Parse scraping interval
	interval, err := time.ParseDuration(config.Scraping.Interval)
	if err != nil {
		log.Fatalf("Invalid scraping interval: %v", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received, stopping...")
		cancel()
	}()

	// Authenticate
	log.Println("Authenticating with KSEM meter...")
	token, err := authenticate(ctx, config)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// Connect to WebSocket
	log.Println("Connecting to WebSocket...")
	conn, err := connectWebSocket(config, token)
	if err != nil {
		log.Fatalf("WebSocket connection failed: %v", err)
	}
	defer conn.Close()

	log.Printf("Receiving data updates (displaying every %s)", interval)

	// Create ticker for periodic output
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastData *KSEMData
	dataChan := make(chan *KSEMData, 1)

	// Goroutine to continuously read from WebSocket
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msgType, message, err := conn.ReadMessage()
				if err != nil {
					log.Printf("Error reading from WebSocket: %v", err)
					cancel()
					return
				}

				if config.Debug {
					log.Printf("Received WebSocket message type: %d, size: %d bytes", msgType, len(message))
				}

				data, err := parseProtobufMessage(message, config)
				if err != nil {
					log.Printf("Error parsing data: %v", err)
					continue
				}

				// Send latest data to channel (non-blocking)
				select {
				case dataChan <- data:
				default:
				}
			}
		}
	}()

	// Main loop: output data at configured intervals
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down gracefully...")
			return

		case data := <-dataChan:
			lastData = data

		case <-ticker.C:
			if lastData != nil {
				if err := outputData(config, lastData); err != nil {
					log.Printf("Error outputting data: %v", err)
				}
			} else {
				log.Println("No data received yet...")
			}
		}
	}
}
