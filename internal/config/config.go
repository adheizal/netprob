package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig  `yaml:"server"`
	Metrics  MetricsConfig `yaml:"metrics"`
	Agent    AgentConfig   `yaml:"agent"`
	LogLevel string        `yaml:"log_level"`
}

type ServerConfig struct {
	Listen        string `yaml:"listen"`
	DataDir       string `yaml:"data_dir"`
	AdminEmail    string `yaml:"admin_email"`
	AdminPassword string `yaml:"admin_password"`
	Port          int    `yaml:"-"`
}

type MetricsConfig struct {
	Backend string `yaml:"backend"` // "sqlite" or "victoriametrics"
	URL     string `yaml:"url"`     // VM URL when backend is victoriametrics
}

type AgentConfig struct {
	ID             string            `yaml:"id"`
	Name           string            `yaml:"name"`
	Controller     string            `yaml:"controller"`
	Token          string            `yaml:"token"`
	Version        string            `yaml:"version"`
	Region         string            `yaml:"region"`
	GeoIPURL       string            `yaml:"geoip_url"`
	PrimaryAddress string            `yaml:"primary_address"`
	Capabilities   map[string]string `yaml:"capabilities"`
	EnvFile        string            `yaml:"env_file"`
	Labels         map[string]string `yaml:"labels"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	}

	applyDefaults(cfg)
	return cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Listen:        ":8080",
			DataDir:       "./data",
			AdminEmail:    "admin@netprob.local",
			AdminPassword: "changeme",
		},
		Metrics: MetricsConfig{
			Backend: "sqlite",
		},
		Agent: AgentConfig{
			GeoIPURL: "http://ip-api.com/json/",
		},
		LogLevel: "info",
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = ":8080"
	}
	if cfg.Server.DataDir == "" {
		cfg.Server.DataDir = "./data"
	}
	if cfg.Server.AdminEmail == "" {
		cfg.Server.AdminEmail = "admin@netprob.local"
	}
	if cfg.Server.AdminPassword == "" {
		cfg.Server.AdminPassword = "changeme"
	}
	if cfg.Metrics.Backend == "" {
		cfg.Metrics.Backend = "sqlite"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	// Override from env
	if v := os.Getenv("NETPROB_SERVER_LISTEN"); v != "" {
		cfg.Server.Listen = v
	}
	if v := os.Getenv("NETPROB_DATA_DIR"); v != "" {
		cfg.Server.DataDir = v
	}
	if v := os.Getenv("NETPROB_ADMIN_EMAIL"); v != "" {
		cfg.Server.AdminEmail = v
	}
	if v := os.Getenv("NETPROB_ADMIN_PASSWORD"); v != "" {
		cfg.Server.AdminPassword = v
	}
	if v := os.Getenv("NETPROB_METRICS_BACKEND"); v != "" {
		cfg.Metrics.Backend = v
	}
	if v := os.Getenv("NETPROB_VM_URL"); v != "" {
		cfg.Metrics.URL = v
	}
	if v := os.Getenv("NETPROB_AGENT_TOKEN"); v != "" {
		cfg.Agent.Token = v
	}
	if v := os.Getenv("NETPROB_AGENT_ID"); v != "" {
		cfg.Agent.ID = v
	}
	if v := os.Getenv("NETPROB_AGENT_NAME"); v != "" {
		cfg.Agent.Name = v
	}
	if v := os.Getenv("NETPROB_AGENT_REGION"); v != "" {
		cfg.Agent.Region = v
	}
	if v := os.Getenv("NETPROB_AGENT_PRIMARY_ADDRESS"); v != "" {
		cfg.Agent.PrimaryAddress = v
	}
	if v, exists := os.LookupEnv("NETPROB_GEOIP_URL"); exists {
		cfg.Agent.GeoIPURL = v
	}
	if v := os.Getenv("NETPROB_CONTROLLER"); v != "" {
		cfg.Agent.Controller = v
	}
}

func (c *Config) DSN() string {
	dsn := c.Server.DataDir + "/netprob.db"
	if c.Server.DataDir == "" {
		dsn = "netprob.db"
	}
	return dsn
}

func (c *Config) SessionTimeout() time.Duration {
	return 30 * time.Second
}
