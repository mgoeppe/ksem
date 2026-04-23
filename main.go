package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mgoeppe/ksem/pkg/obis"
	"github.com/mgoeppe/ksem/pkg/output"
	outputjson "github.com/mgoeppe/ksem/pkg/output/json"
	"github.com/mgoeppe/ksem/pkg/output/sqlite"
	"github.com/mgoeppe/ksem/pkg/output/tui"
	pb "github.com/mgoeppe/ksem/pkg/proto"
	"github.com/mgoeppe/ksem/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"
)

// Config holds all configuration settings
type Config struct {
	Meter struct {
		Host     string `mapstructure:"host"`
		Password string `mapstructure:"password"`
	} `mapstructure:"meter"`
	Output struct {
		Format   string `mapstructure:"format"`
		FilePath string `mapstructure:"file_path"`
		Interval string `mapstructure:"interval"`
	} `mapstructure:"output"`
	Debug bool `mapstructure:"debug"`
}

func loadConfig(filename string) (*Config, error) {
	if filename != "" {
		viper.SetConfigFile(filename)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}

	// Set defaults
	viper.SetDefault("output.format", "tui")
	viper.SetDefault("output.interval", "1m")
	viper.SetDefault("debug", false)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func authenticate(ctx context.Context, config *Config) (*oauth2.Token, error) {
	tokenURL := fmt.Sprintf("http://%s/api/web-login/token", config.Meter.Host)

	// KSEM OAuth2 constants
	const (
		clientID     = "emos"
		clientSecret = "56951025"
		username     = "admin"
	)

	oauth2Config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: tokenURL,
		},
	}

	token, err := oauth2Config.PasswordCredentialsToken(ctx, username, config.Meter.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain token: %w", err)
	}

	if config.Debug {
		log.Debugf("Authentication successful, token type: %s", token.TokenType)
	}

	return token, nil
}

func connectWebSocket(config *Config, token *oauth2.Token) (*websocket.Conn, error) {
	// Build WebSocket URL with hardcoded config_id for sumvalues endpoint
	const configID = "kostal-energyflow/sumvalues"
	wsURL := url.URL{
		Scheme: "ws",
		Host:   config.Meter.Host,
		Path:   fmt.Sprintf("/api/data-transfer/ws/protobuf/gdr/local/values/%s", configID),
	}

	if config.Debug {
		log.Debugf("Connecting to WebSocket: %s", wsURL.String())
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
		log.Info("WebSocket connected and authenticated")
	}

	return conn, nil
}

func parseProtobufMessage(data []byte, config *Config) (*types.KSEMData, error) {
	if config.Debug {
		log.Debugf("Received %d bytes of protobuf data", len(data))
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

	// Debug logging for raw values
	if config.Debug {
		if len(allValues) > 0 {
			log.Debugf("Parsed %d OBIS values from protobuf message", len(allValues))
			for obisHex, value := range allValues {
				if code, ok := obis.Lookup(obisHex); ok {
					log.Debugf("  %s = %s", code.Description, code.Format(value))
				} else {
					log.Debugf("  OBIS 0x%X = %d (unknown)", obisHex, value)
				}
			}
		}
		if len(allFlexValues) > 0 {
			log.Debugf("Parsed %d flex values from protobuf message", len(allFlexValues))
			for key, value := range allFlexValues {
				log.Debugf("  Flex[%s] = %d (%.2f W)", key, value, float64(value)/1000.0)
			}
		}
	}

	// Parse all values using OBIS package
	parsed := obis.ParseValues(allValues, allFlexValues, config.Debug)

	// Build result structure
	result := &types.KSEMData{
		Timestamp:              time.Now(),
		ActivePowerTotal:       parsed.ActivePowerTotal,
		ActivePowerL1:          parsed.ActivePowerL1,
		ActivePowerL2:          parsed.ActivePowerL2,
		ActivePowerL3:          parsed.ActivePowerL3,
		GridFrequency:          parsed.GridFrequency,
		EnergyGridPurchase:     parsed.EnergyGridPurchase,
		EnergyGridFeedIn:       parsed.EnergyGridFeedIn,
		EnergySolarTotal:       parsed.EnergySolarTotal,
		EnergyBatteryCharge:    parsed.EnergyBatteryCharge,
		EnergyBatteryDischarge: parsed.EnergyBatteryDischarge,
		EnergyWallbox:          parsed.EnergyWallbox,
		PowerSolar:             parsed.PowerSolar,
		PowerBattery:           parsed.PowerBattery,
		PowerGrid:              parsed.PowerGrid,
		PowerHome:              parsed.PowerHome,
		PowerWallbox:           parsed.PowerWallbox,
		BatterySOC:             parsed.BatterySOC,
	}

	return result, nil
}

// runOutputHandler starts the output handler with channel-based data flow
func runOutputHandler(ctx context.Context, conn *websocket.Conn, config *Config, handler output.Handler) {
	dataChan := make(chan *types.KSEMData, 10)
	errChan := make(chan error, 1)

	// Start websocket reader goroutine
	go readWebSocket(ctx, conn, config, dataChan, errChan)

	// Start output handler
	if err := handler.Run(ctx, dataChan, errChan); err != nil {
		log.Errorf("Output handler error: %v", err)
	}
	log.Info("Shutting down gracefully...")
}

// readWebSocket reads from websocket and sends data to channels
func readWebSocket(ctx context.Context, conn *websocket.Conn, config *Config, dataChan chan<- *types.KSEMData, errChan chan<- error) {
	defer close(dataChan)
	defer close(errChan)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msgType, message, err := conn.ReadMessage()
			if err != nil {
				errChan <- fmt.Errorf("websocket read error: %w", err)
				return
			}

			if config.Debug {
				log.Debugf("Received WebSocket message type: %d, size: %d bytes", msgType, len(message))
			}

			data, err := parseProtobufMessage(message, config)
			if err != nil {
				log.Errorf("Error parsing data: %v", err)
				continue
			}

			// Send data to channel (non-blocking)
			select {
			case dataChan <- data:
			case <-ctx.Done():
				return
			default:
				// Channel full, skip this update
			}
		}
	}
}

func main() {
	// Define command line flags using pflag
	pflag.StringP("config", "c", "config.yaml", "Path to configuration file")
	pflag.String("host", "", "KSEM meter hostname or IP address")
	pflag.String("password", "", "KSEM meter admin password")
	pflag.StringP("format", "f", "", "Output format (tui, json, or sqlite)")
	pflag.StringP("output", "o", "", "Output file path (for JSON and SQLite formats)")
	pflag.StringP("interval", "i", "", "Output interval as Go duration (e.g., 1s, 5m, 1h) for JSON and SQLite formats")
	pflag.BoolP("debug", "d", false, "Enable debug mode")
	pflag.Parse()

	// Configure logrus with colors
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
	log.SetOutput(os.Stdout)

	// Get config file path from flags (don't bind other flags to viper to avoid conflicts)
	configFile, _ := pflag.CommandLine.GetString("config")

	// Load configuration
	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override config with command line flags (flags take precedence)
	// Use pflag directly instead of viper to avoid conflicts with config structure
	if pflag.CommandLine.Changed("host") {
		config.Meter.Host, _ = pflag.CommandLine.GetString("host")
	}
	if pflag.CommandLine.Changed("password") {
		config.Meter.Password, _ = pflag.CommandLine.GetString("password")
	}
	if pflag.CommandLine.Changed("format") {
		config.Output.Format, _ = pflag.CommandLine.GetString("format")
	}
	if pflag.CommandLine.Changed("output") {
		config.Output.FilePath, _ = pflag.CommandLine.GetString("output")
	}
	if pflag.CommandLine.Changed("interval") {
		config.Output.Interval, _ = pflag.CommandLine.GetString("interval")
	}
	if pflag.CommandLine.Changed("debug") {
		config.Debug, _ = pflag.CommandLine.GetBool("debug")
	}

	if config.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug mode enabled")
	} else {
		log.SetLevel(log.InfoLevel)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		if config.Output.Format != "tui" {
			log.Info("Shutdown signal received, stopping...")
		}
		cancel()
	}()

	// Authenticate
	if config.Output.Format != "tui" {
		log.Info("Authenticating with KSEM meter...")
	}
	token, err := authenticate(ctx, config)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// Connect to WebSocket
	if config.Output.Format != "tui" {
		log.Info("Connecting to WebSocket...")
	}
	conn, err := connectWebSocket(config, token)
	if err != nil {
		log.Fatalf("WebSocket connection failed: %v", err)
	}
	defer conn.Close()

	// Create output handler based on configuration
	var handler output.Handler
	switch config.Output.Format {
	case "tui":
		handler = tui.NewHandler()

	case "json":
		log.Info("Starting JSON output mode...")
		handler = outputjson.NewHandler(config.Output.FilePath, config.Output.Interval)

	case "sqlite":
		if config.Output.FilePath == "" {
			log.Fatal("SQLite output requires --output or file_path in config")
		}
		log.Infof("Starting SQLite output mode (database: %s)...", config.Output.FilePath)
		handler = sqlite.NewHandler(config.Output.FilePath, config.Output.Interval)

	default:
		log.Fatalf("Unknown output format: %s (supported: tui, json, sqlite)", config.Output.Format)
	}

	// Run the output handler
	runOutputHandler(ctx, conn, config, handler)
}
