package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/itoooak/miniflux-utils/internal/server"
	"github.com/itoooak/miniflux-utils/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	host = flag.String("host", "localhost", "host to listen on")
	port = flag.String("port", "8080", "port to listen on")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "This program runs the Miniflux MCP server over SSE HTTP.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEndpoints:\n")
		fmt.Fprintf(os.Stderr, "  /miniflux - Miniflux MCP server\n")
		os.Exit(1)
	}
	flag.Parse()

	addr := fmt.Sprintf("%s:%s", *host, *port)

	cfg, err := utils.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logger := log.New(os.Stderr, "[miniflux-mcp-http] ", log.LstdFlags|log.Lshortfile)

	srv, err := server.NewServer(cfg, logger)
	if err != nil {
		logger.Fatalf("Failed to create server: %v", err)
	}

	logger.Printf("MCP server serving at %s", addr)
	handler := mcp.NewSSEHandler(func(request *http.Request) *mcp.Server {
		url := request.URL.Path
		logger.Printf("Handling request for URL %s\n", url)
		switch url {
		case "/miniflux":
			return srv.GetMCPServer()
		default:
			return nil
		}
	}, nil)

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
