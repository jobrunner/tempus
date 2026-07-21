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
	Server    ServerConfig    `mapstructure:"server"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
	Tracing   TracingConfig   `mapstructure:"tracing"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Cache     CacheConfig     `mapstructure:"cache"`
	Providers ProvidersConfig `mapstructure:"providers"`
	Query     QueryConfig     `mapstructure:"query"`
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

type CacheConfig struct {
	Type string `mapstructure:"type"` // disk|memory|redis
	Path string `mapstructure:"path"`
}

type QueryConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
}

type ProvidersConfig struct {
	OpenMeteo OpenMeteoConfig `mapstructure:"openmeteo"`
}

type OpenMeteoConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	ArchiveBaseURL  string        `mapstructure:"archive_base_url"`
	ForecastBaseURL string        `mapstructure:"forecast_base_url"`
	Timeout         time.Duration `mapstructure:"timeout"`
	ArchiveDelay    time.Duration `mapstructure:"archive_delay"`
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

	viper.SetDefault("cache.type", "disk")
	viper.SetDefault("cache.path", "./data/cache.bolt")
	viper.SetDefault("query.timeout", 30*time.Second)
	viper.SetDefault("providers.openmeteo.enabled", true)
	viper.SetDefault("providers.openmeteo.archive_base_url", "https://archive-api.open-meteo.com/v1/archive")
	viper.SetDefault("providers.openmeteo.forecast_base_url", "https://api.open-meteo.com/v1/forecast")
	viper.SetDefault("providers.openmeteo.timeout", 10*time.Second)
	viper.SetDefault("providers.openmeteo.archive_delay", 5*24*time.Hour)
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
