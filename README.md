# KSEM Meter Scraper

A Go tool for scraping real-time energy data from Kostal KSEM (Kostal Smart Energy Meter) devices. Monitors solar production, battery status, grid power, and home consumption through WebSocket connections with Protocol Buffer encoding.

## Features

✅ **OAuth2 Authentication** - Automatic token management
✅ **WebSocket Connection** - Real-time data streaming
✅ **Protocol Buffer Decoding** - Efficient binary message parsing
✅ **Power Flow Monitoring** - Solar, battery, grid, home, and wallbox
✅ **Directional Indicators** - Shows charging/discharging, importing/exporting
✅ **Battery State of Charge** - Real-time SOC percentage
✅ **OBIS Code Support** - Standardized energy measurement codes
✅ **Multiple Output Formats** - Console display or JSON file

## Quick Start

```bash
# Clone and build
git clone https://github.com/matoubidou/ksem.git
cd ksem
go build -o ksem

# Configure
cp config.yaml.example config.yaml
# Edit config.yaml with your KSEM host and password

# Run
./ksem
```

## Configuration

Create `config.yaml` from the example:

```yaml
meter:
  host: "ksem.fritz.box"  # Your KSEM hostname or IP
  password: "your-password-here"

scraping:
  interval: "5s"          # Display update interval

output:
  format: "console"       # "console" or "json"
  file_path: ""           # Optional JSON file path

debug: false
```

**Note:** OAuth2 credentials (client_id, client_secret, username) and the WebSocket endpoint are hardcoded as they're constant for all KSEM devices.

## Sample Output

```
=== KSEM Data at 2025-12-02 13:36:29 ===

--- Instantaneous Power Flows ---
Solar Production:   951.00 W
Battery:            +587.00 W (charging)
Battery SOC:        15%
Grid:               -0.30 W (exporting)
Home Consumption:   291.43 W
Wallbox:            0.00 W
```

### Power Flow Indicators

- **Solar Production**: Always positive (power generated)
- **Battery**:
  - Positive (+) = charging
  - Negative (-) = discharging
  - Zero = idle
- **Grid**:
  - Positive (+) = importing from grid
  - Negative (-) = exporting to grid
- **Home Consumption**: Always positive (power consumed)
- **Wallbox**: Always positive (EV charging power)

## Architecture

### Authentication
- **Protocol**: OAuth2 (Resource Owner Password Credentials)
- **Credentials**: Hardcoded (client_id: `emos`, client_secret: `56951025`, username: `admin`)
- **Token**: Bearer JWT with 7-day expiration

### Data Protocol
- **Transport**: WebSocket over HTTP
- **Endpoint**: `ws://host/api/data-transfer/ws/protobuf/gdr/local/values/kostal-energyflow/sumvalues`
- **Encoding**: Protocol Buffers (binary)
- **Update Rate**: ~1 Hz (real-time)
### Message Structure

Protocol Buffer messages contain:
- **GDRs**: Container with multiple Grid Data Records
- **GDR**: Individual data record with:
  - `values`: Map of OBIS codes (uint64) to measurements (uint64)
  - `flexValues`: Map of power flow keys (string) to values (int64)
  - `timestamp`: Measurement time
  - `status`: Device status

### Flex Values (Power Flows)
- `pvPowerTotal`: Solar production (mW, sign inverted)
- `batteryPowerTotal`: Battery power (mW, +charging/-discharging)
- `gridPowerTotal`: Grid power (mW, +importing/-exporting)
- `housePowerTotal`: Home consumption (mW)
- `wallboxPowerTotal`: Wallbox consumption (mW)
- `systemStateOfCharge`: Battery SOC (%)

### OBIS Codes (Cumulative Totals)

| OBIS Code      | Hex           | Description                    |
| -------------- | ------------- | ------------------------------ |
| 1-0:1.4.0*255  | 0x100010400FF | Total active power (mW)        |
| 1-0:14.4.0*255 | 0x1000E0400FF | Grid frequency (mHz)           |
| 1-0:1.8.0*255  | 0x100010800FF | Grid energy purchase (mWh)     |
| 1-0:2.8.0*255  | 0x100020800FF | Grid energy feed-in (mWh)      |
| 1-0:65.8.0*255 | 0x100410800FF | Solar total energy (mWh)       |
| 1-0:67.8.0*255 | 0x100430800FF | Battery charge energy (mWh)    |
| 1-0:68.8.0*255 | 0x100440800FF | Battery discharge energy (mWh) |
| 1-0:74.8.0*255 | 0x1004A0800FF | Wallbox energy (mWh)           |

Values are in milli-units (mW, mWh, mHz) and automatically converted to standard units.

## Project Structure

```
ksem/
├── main.go              # Main application
├── proto/
│   ├── ksem.proto       # Protocol Buffer schema
│   ├── ksem.pb.go       # Generated protobuf code
│   └── generate.go      # go:generate directive
├── obis/
│   └── obis.go          # OBIS code library with metadata
├── config.yaml          # Your configuration (not committed)
├── config.yaml.example  # Configuration template
└── README.md
```

## Development

### Regenerate Protocol Buffer Code

```bash
go generate ./proto
```

Or manually:
```bash
protoc --go_out=. --go_opt=paths=source_relative proto/ksem.proto
```

### Adding New OBIS Codes

Edit `obis/obis.go` and add to the registry:

```go
var NewCode = Code{
    Hex:         0xYOURHEXVALUE,
    Description: "Your description",
    Unit:        Watt,
    ScaleFactor: 0.001,
}
```

## Troubleshooting

**Connection Refused**
- Verify KSEM is reachable: `ping ksem.fritz.box`
- Check firewall allows WebSocket on port 80

**Authentication Failed**
- Verify password in `config.yaml`
- Password is case-sensitive
- Special characters must be properly quoted in YAML

**No Data Received**
- Enable debug mode: `debug: true` in config.yaml
- Check WebSocket messages are arriving
- Verify timestamp in debug logs is updating

**Incorrect Values**
- Values are automatically converted from milli-units
- Negative values indicate direction (export/discharge)
- Check battery SOC is percentage (0-100)

## Dependencies

- `github.com/gorilla/websocket` - WebSocket client
- `github.com/spf13/viper` - Configuration management
- `golang.org/x/oauth2` - OAuth2 authentication
- `google.golang.org/protobuf` - Protocol Buffer support

## License

MIT

## Acknowledgments

Developed through reverse engineering of the KSEM web application to discover the WebSocket + Protocol Buffer architecture. Thanks to the Kostal team for building a modern, real-time energy monitoring system.
