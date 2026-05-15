# miniflux-utils

Utilities for Miniflux including MCP server and CLI tool.

## Configuration

### Common

Required:
- `MINIFLUX_URL` - Miniflux instance URL
- `MINIFLUX_API_TOKEN` - Miniflux API token

Optional:
- `MINIFLUX_TIMEOUT` - Miniflux client timeout (default: `30s`)

### MCP Server Only

Content conversion:
- `CONVERTER_TIMEOUT` - HTML to Markdown converter timeout (default: `30s`)

Caching:
- `MCP_CACHE_MODE` - Cache mode: `memory`, `disk`, or `both` (default: `disk`)
- `MCP_CACHE_TTL` - Cache time-to-live (default: `24h`)
- `MCP_CACHE_MAX_SIZE` - Cache max size in bytes (default: `1GB`)
- `MCP_CACHE_PATH` - Disk cache path (default: `{os.TempDir()}/miniflux-mcp-cache`)
