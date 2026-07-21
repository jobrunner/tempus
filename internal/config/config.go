package config

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// EnvPrefix is the prefix for all environment variables. Config key "server.host"
// is read from TEMPUS_SERVER_HOST.
const EnvPrefix = "TEMPUS"

// Config is the whole service configuration. Add sub-structs per concern.
type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Logging LoggingConfig `mapstructure:"logging"`
	Metrics MetricsConfig `mapstructure:"metrics"`
	Tracing TracingConfig `mapstructure:"tracing"`
	Auth    AuthConfig    `mapstructure:"auth"`
}

type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`  // debug|info|warn|error
	Format string `mapstructure:"format"` // json|text
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

type TracingConfig struct {
	Enabled     bool    `mapstructure:"enabled"`
	Endpoint    string  `mapstructure:"endpoint"`  // host:port; empty disables OTLP push
	Transport   string  `mapstructure:"transport"` // http|grpc
	SampleRatio float64 `mapstructure:"sample_ratio"`
}

type AuthConfig struct {
	Token string `mapstructure:"-"` // secret: loaded from env directly, never from file
}

// Defaults registers every default. Call before Load (and from cmd initConfig).
func Defaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", 30*time.Second)
	viper.SetDefault("server.shutdown_timeout", 15*time.Second)

	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	viper.SetDefault("metrics.enabled", false)
	viper.SetDefault("metrics.port", 2112)
	viper.SetDefault("metrics.path", "/metrics")

	viper.SetDefault("tracing.enabled", false)
	viper.SetDefault("tracing.endpoint", "")
	viper.SetDefault("tracing.transport", "http")
	viper.SetDefault("tracing.sample_ratio", 1.0)
}

// Load merges defaults, an optional config file, and environment variables.
func Load(configPath string) (*Config, error) {
	Defaults()

	viper.SetEnvPrefix(EnvPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // server.host -> TEMPUS_SERVER_HOST
	viper.AutomaticEnv()

	if configPath != "" {
		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Secrets never go through the config file (so they can't leak in a viper
	// debug dump). Read them straight from the environment.
	cfg.Auth.Token = os.Getenv(EnvPrefix + "_AUTH_TOKEN")

	return &cfg, nil
}
