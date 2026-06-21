package config

import (
	"log"

	"github.com/kelseyhightower/envconfig"
)

type Settings struct {
	DataPath     string   `envconfig:"DATA_PATH" default:"/app/data"`
	BackupsPath  string   `envconfig:"BACKUPS_PATH" default:""`
	// Database is a URL-style connection string covering driver, credentials,
	// host, and database name. Empty means "use SQLite at DataPath" (default
	// behavior, fully backwards compatible). See docs/databases.md.
	Database     string   `envconfig:"DATABASE" default:""`
	K8sNamespace string   `envconfig:"K8S_NAMESPACE" default:"claworc"`
	DockerHost   string   `envconfig:"DOCKER_HOST" default:""`
	AuthDisabled bool     `envconfig:"AUTH_DISABLED" default:"false"`
	RPOrigins    []string `envconfig:"RP_ORIGINS" default:"http://localhost:8000"`
	RPID         string   `envconfig:"RP_ID" default:"localhost"`

	// Terminal session settings
	TerminalHistoryLines   int    `envconfig:"TERMINAL_HISTORY_LINES" default:"1000"`
	TerminalRecordingDir   string `envconfig:"TERMINAL_RECORDING_DIR" default:""`
	TerminalSessionTimeout string `envconfig:"TERMINAL_SESSION_TIMEOUT" default:"30m"`

	// LLM gateway settings
	LLMGatewayPort int    `envconfig:"LLM_GATEWAY_PORT" default:"40001"`
	LLMResponseLog string `envconfig:"LLM_RESPONSE_LOG" default:""`

	// Claude subscription (Claude Code OAuth) settings. The gateway reads the
	// `claude` CLI credentials file to authenticate anthropic-oauth providers
	// with a shared subscription. See docs/virtual-keys.md.
	// ClaudeConfigDir: directory holding `.credentials.json` (default ~/.claude).
	ClaudeConfigDir string `envconfig:"CLAUDE_CONFIG_DIR" default:""`
	// ClaudeRefreshCmd: command the gateway runs to refresh the subscription
	// token when it nears expiry (the `claude` CLI owns the refresh). Empty
	// disables on-demand refresh — rely on an external keep-alive instead.
	ClaudeRefreshCmd string `envconfig:"CLAUDE_REFRESH_CMD" default:""`
}

var Cfg Settings

func Load() {
	if err := envconfig.Process("CLAWORC", &Cfg); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
}
