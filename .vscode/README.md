# VS Code MCP Configuration

The `.vscode/mcp.json` file contains your Grafana credentials and is gitignored for security.

## Setup

1. Copy the template:
   ```bash
   cp .vscode/mcp.json.example .vscode/mcp.json
   ```

2. Edit `.vscode/mcp.json` and replace:
   - `your-server:3000` with your Grafana server URL
   - `your-service-account-token-here` with your actual Grafana service account token

3. Get a Grafana Service Account Token:
   - In Grafana: Administration → Service Accounts → Create service account
   - Add token with Viewer or Editor role
   - Copy the token to your `mcp.json`

4. Restart VS Code to activate the MCP server
