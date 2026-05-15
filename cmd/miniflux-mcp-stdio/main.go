package main

import (
	"context"
	"log"
	"os"

	"github.com/itoooak/miniflux-utils/internal/server"
	"github.com/itoooak/miniflux-utils/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	cfg, err := utils.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logger := log.New(os.Stderr, "[miniflux-mcp-stdio] ", log.LstdFlags|log.Lshortfile)

	srv, err := server.NewServer(cfg, logger)
	if err != nil {
		logger.Fatalf("Failed to create server: %v", err)
	}

	transport := &mcp.StdioTransport{}

	ctx := context.Background()

	logger.Println("Starting Miniflux MCP Server (stdio transport)")
	if err := srv.Run(ctx, transport); err != nil {
		logger.Fatalf("Server error: %v", err)
	}

	if err := srv.Shutdown(ctx); err != nil {
		logger.Printf("Shutdown error: %v", err)
	}
}
