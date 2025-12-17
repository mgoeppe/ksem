# KSEM Docker Deployment

## Quick Start

1. **Prepare your configuration:**
   ```bash
   # Copy example config
   cp config.yaml.example config.yaml

   # Edit with your KSEM credentials
   nano config.yaml
   ```

2. **Set SQLite output in config.yaml:**
   ```yaml
   output:
     format: sqlite
     file_path: "/app/data/ksem.db"
     interval: "1m"
   ```

3. **Create data directory:**
   ```bash
   mkdir -p data
   ```

4. **Build the Docker image:**
   ```bash
   docker build -t ksem:latest .
   ```

5. **Run the container:**
   ```bash
   docker run -d \
     --name ksem-monitor \
     -v $(pwd)/config.yaml:/app/config/config.yaml:ro \
     -v $(pwd)/data:/app/data \
     -e CONFIG_PATH=/app/config/config.yaml \
     -e SQLITE_PATH=/app/data/ksem.db \
     --restart unless-stopped \
     ksem:latest
   ```

## Custom Paths

### Using Environment Variables

```bash
docker run -d \
  --name ksem-monitor \
  -v /path/to/your/config.yaml:/app/config/config.yaml:ro \
  -v /path/to/data:/app/data \
  -e CONFIG_PATH=/app/config/config.yaml \
  -e SQLITE_PATH=/app/data/ksem.db \
  ksem:latest
```

## Custom Paths with Absolute Paths

```bash
# Build locally
docker build -t ksem:latest .

# Build with specific platform
docker build --platform linux/amd64 -t ksem:latest .
```

## Viewing Logs

```bash
# Follow logs
docker logs -f ksem-monitor

# View last 100 lines
docker logs --tail=100 ksem-monitor
```

## Stopping the Container

```bash
docker stop ksem-monitor
docker rm ksem-monitor
```

## Accessing the SQLite Database

```bash
# Install sqlite3 if needed
apt-get install sqlite3

# Query the database
sqlite3 data/ksem.db "SELECT * FROM ksem_data ORDER BY timestamp DESC LIMIT 10;"

# Or use docker to query
docker exec ksem-monitor sqlite3 /app/data/ksem.db "SELECT * FROM ksem_data ORDER BY timestamp DESC LIMIT 10;"
```

## Configuration Example

Your `config.yaml` should look like:

```yaml
meter:
  host: ksem.fritz.box
  password: "your-password-here"

output:
  format: sqlite
  file_path: "/app/data/ksem.db"
  interval: "1m"

debug: false
```

## Troubleshooting

### Container exits immediately
Check logs: `docker logs ksem-monitor`

### Cannot connect to KSEM
- Ensure KSEM is reachable from Docker network
- Try using IP address instead of hostname
- Check firewall settings

### SQLite database not persisting
- Ensure `data` directory exists and has proper permissions
- Check volume mount in docker run command

### Permission errors
```bash
# Fix permissions
chmod 755 data
```
