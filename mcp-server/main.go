// Command claworc-mcp is a Model Context Protocol (MCP) server that exposes the
// Claworc control-plane REST API as a set of tools, so an LLM client (Claude
// Code, Claude Desktop, etc.) can fully manage a Claworc deployment — creating
// and operating OpenClaw instances, managing users, teams, LLM providers,
// skills, backups, Kanban boards, and global settings.
//
// It communicates over stdio (the standard MCP transport for local use) and
// authenticates to the control plane either via a bearer API token or with a
// username/password session cookie (fallback).
//
// Configuration (environment variables):
//
//	CLAWORC_URL       Base URL of the control plane (default http://localhost:8000)
//	CLAWORC_TOKEN     Bearer API token (preferred; claworc_pat_… tokens from the dashboard)
//	CLAWORC_USERNAME  Login username (fallback when CLAWORC_TOKEN is not set)
//	CLAWORC_PASSWORD  Login password (fallback when CLAWORC_TOKEN is not set)
//	CLAWORC_INSECURE  Set to "true" to skip TLS certificate verification
//	CLAWORC_TIMEOUT   Per-request timeout (Go duration, default 60s)
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gluk-w/claworc/mcp-server/internal/client"
	"github.com/gluk-w/claworc/mcp-server/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	// MCP servers speak JSON-RPC over stdout, so all diagnostics must go to
	// stderr to avoid corrupting the protocol stream.
	log.SetOutput(os.Stderr)
	log.SetPrefix("[claworc-mcp] ")
	log.SetFlags(0)

	cfg := client.Config{
		BaseURL:  getenv("CLAWORC_URL", "http://localhost:8000"),
		Token:    os.Getenv("CLAWORC_TOKEN"),
		Username: os.Getenv("CLAWORC_USERNAME"),
		Password: os.Getenv("CLAWORC_PASSWORD"),
		Insecure: os.Getenv("CLAWORC_INSECURE") == "true",
		Timeout:  parseTimeout(os.Getenv("CLAWORC_TIMEOUT"), 60*time.Second),
	}

	c, err := client.New(cfg)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	// Verify connectivity and credentials up front so misconfiguration is
	// reported immediately rather than on the first tool call.
	// With a bearer token there is no pre-flight login; errors surface on the
	// first tool call instead (avoids an extra round-trip at startup).
	if cfg.Token == "" {
		loginCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		if err := c.EnsureLogin(loginCtx); err != nil {
			cancel()
			log.Fatalf("failed to authenticate to %s: %v", cfg.BaseURL, err)
		}
		cancel()
		log.Printf("authenticated to %s", cfg.BaseURL)
	} else {
		log.Printf("using bearer token auth for %s", cfg.BaseURL)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "claworc",
		Version: version,
	}, nil)

	tools.Register(server, c)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("starting MCP server on stdio")
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseTimeout(v string, def time.Duration) time.Duration {
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("invalid CLAWORC_TIMEOUT %q, using %s", v, def)
		return def
	}
	return d
}
