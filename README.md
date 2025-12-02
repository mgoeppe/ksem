# KSEM Meter Scraper

A Go tool for scraping real-time data from Kostal KSEM (Kostal Smart Energy Meter) devices via WebSocket and Protocol Buffers.

## Status

✅ **OAuth2 Authentication** - Working  
✅ **WebSocket Connection** - Working  
✅ **Data Reception** - Working (receiving ~1 message/second, ~830 bytes each)  
🚧 **Protocol Buffer Decoding** - In Progress

The application successfully connects to the KSEM meter and receives real-time data updates via WebSocket with Protocol Buffer encoding.

## Architecture Discovered

The KSEM meter uses a modern real-time architecture:

### Authentication
- **Protocol**: OAuth2 Resource Owner Password Credentials flow
- **Client ID**: `emos`
- **Client Secret**: `56951025`
- **Token endpoint**: `http://ksem.fritz.box/api/web-login/token`
- **Token type**: Bearer JWT (7-day expiration)

### Data Protocol
- **Protocol**: WebSocket with Protocol Buffer binary encoding
- **WebSocket URL**: `ws://ksem.fritz.box/api/data-transfer/ws/protobuf/gdr/local/values/{config-id}`
- **Config ID**: `smart-meter` (for main smart meter data)
- **Authentication**: Bearer token sent as first WebSocket message
- **Message format**: Binary Protocol Buffers (GDRs/GDR message types)
- **Update frequency**: ~1 Hz (once per second)
- **Message size**: ~830 bytes per message

### Why HTTP REST Failed
The meter does NOT expose data via HTTP REST endpoints. All data endpoints (`/api/dxs.json`, etc.) return 502 Bad Gateway because they don't exist. Data is exclusively available via WebSocket.

## Prerequisites

- Go 1.23 or higher
- Network access to your Kostal KSEM meter
- KSEM meter credentials (default username: admin)

## Installation

```bash
# Clone the repository
git clone https://github.com/matoubidou/ksem.git
cd ksem

# Download dependencies
go mod download

# Build the application
go build -o ksem
```

## Configuration

Edit `config.yaml` to match your setup:

```yaml
meter:
  host: "ksem.fritz.box"  # Your KSEM meter hostname or IP
  username: "admin"        # Default username
  password: "your-password-here"

oauth2:
  client_id: "emos"        # Default OAuth2 client ID
  client_secret: "56951025" # Default OAuth2 client secret

scraping:
  interval: "10s"          # How often to display data
  config_id: "smart-meter" # WebSocket endpoint identifier

output:
  format: "console"        # Output format: "console" or "json"
  file_path: ""            # Optional: path to JSON output file

debug: true                # Enable debug logging
```

## Usage

```bash
./ksem
```

The tool will:
1. Authenticate with the KSEM meter using OAuth2
2. Connect to the WebSocket endpoint
3. Continuously receive and decode meter data
4. Display data at the configured interval

Press `Ctrl+C` to gracefully shut down.

## Data Fields

The tool collects the following measurements using OBIS codes:

| Field                  | OBIS Code     | DXS ID   | Description                |
|------------------------|---------------|----------|----------------------------|
| ActivePowerTotal       | 1-0:1.4.0*255 | 67109120 | Total active power (W)     |
| ActivePowerL1          | 1-0:21.4.0*255| 67109376 | Active power phase L1 (W)  |
| ActivePowerL2          | 1-0:41.4.0*255| 67109632 | Active power phase L2 (W)  |
| ActivePowerL3          | 1-0:61.4.0*255| 67109888 | Active power phase L3 (W)  |
| GridFrequency          | 1-0:14.4.0*255| 16780288 | Grid frequency (Hz)        |
| ActiveEnergyImport     | 1-0:1.8.0*255 | 67371264 | Imported energy (kWh)      |
| ActiveEnergyExport     | 1-0:2.8.0*255 | 67371520 | Exported energy (kWh)      |

## Protocol Buffer Message Structure

Based on analysis of the KSEM web application JavaScript, the Protocol Buffer messages use these types:

- **GDRs**: Container for multiple Grid Data Records
  - Contains a map of GDR messages keyed by device ID
  - Includes a configUuid field

- **GDR**: Single Grid Data Record
  - `id`: Device identifier
  - `status`: Status code (0=UNKNOWN, 1=OK, 2=WARNING, 3=ERROR)
  - `timestamp`: Measurement timestamp
  - `values`: Map of OBIS codes (uint64) to measurements (uint64)
  - `flexValues`: Map of flexible values (string keys to int/string values)

Values are typically encoded in milli-units (mW, mWh, mHz) and must be divided by 1000 to get standard units.

## Development Notes

### WebSocket Implementation

The WebSocket connection requires:
1. OAuth2 authentication to obtain Bearer token
2. WebSocket connection to `ws://host/api/data-transfer/ws/protobuf/gdr/local/values/{config-id}`
3. Immediate authentication by sending Bearer token as text message
4. Continuous reading of binary Protocol Buffer messages

### Protocol Buffer Decoding

The protobuf schema was reverse-engineered from the JavaScript app. The full schema is in `ksem.proto`. To regenerate Go code:

```bash
protoc --go_out=. --go_opt=paths=source_relative ksem.proto
```

Note: Proper protobuf decoding is still being implemented. Currently the tool receives messages but doesn't parse them yet.

Common dxsId values:
- `16780032`: Grid power total (W)
- `33556736`: Total production (kWh)

## Troubleshooting

### Connection Refused
- Ensure the KSEM meter is reachable on your network
- Verify the hostname/IP in `config.yaml`
- Check that port 80 (HTTP/WebSocket) is accessible

### Authentication Failed
- Verify your password in `config.yaml`
- Special characters in password must be properly encoded
- Default username is `admin`

### No Data Received
- Check the `config_id` setting (should be `smart-meter`)
- Enable debug mode to see WebSocket messages
- Verify WebSocket binary messages are being received

## Dependencies

- `golang.org/x/oauth2` - OAuth2 authentication
- `github.com/gorilla/websocket` - WebSocket client
- `google.golang.org/protobuf` - Protocol Buffer support
- `gopkg.in/yaml.v3` - YAML configuration parsing

## License

MIT

## Acknowledgments

This tool was developed through analysis of the KSEM web application's JavaScript code to discover the WebSocket + Protocol Buffer architecture.

