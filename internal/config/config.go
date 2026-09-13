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
	Host            string          `mapstructure:"host"`
	Port            int             `mapstructure:"port"`
	ReadTimeout     time.Duration   `mapstructure:"read_timeout"`
	ShutdownTimeout time.Duration   `mapstructure:"shutdown_timeout"`
	CORS            CORSConfig      `mapstructure:"cors"`
	RateLimit       RateLimitConfig `mapstructure:"rate_limit"`
}

// RateLimitConfig caps requests per client IP on /api/v1. Disabled by default:
// it is meant for a tempus exposed directly on a public IP, without a
// rate-limiting gateway in front.
type RateLimitConfig struct {
	Enabled bool    `mapstructure:"enabled"`
	Rate    float64 `mapstructure:"rate"`  // sustained requests per second per client IP
	Burst   int     `mapstructure:"burst"` // token-bucket depth per client IP
	// TrustedProxies are CIDRs of front proxies/load balancers. When the direct
	// peer is within one, the client IP is taken from X-Forwarded-For (right-most
	// non-trusted entry); otherwise the direct peer is used. Empty (the default)
	// = never trust forwarded headers.
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}

// CORSConfig lists the browser origins allowed to call the API from another
// site. Empty (the default) means no CORS headers are sent at all — the
// service then behaves exactly as it did before CORS existed, which is correct
// for the bundled frontend because that is served from the same origin.
type CORSConfig struct {
	// AllowedOrigins holds exact origins ("https://example.com") and wildcard
	// patterns ("https://*.example.com"). Set via
	// TEMPUS_SERVER_CORS_ALLOWED_ORIGINS as a comma-separated list.
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// Enabled reports whether any origin is allowed; the middleware is only wired
// in when this is true.
func (c CORSConfig) Enabled() bool { return len(c.AllowedOrigins) > 0 }

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
	Batch   BatchConfig   `mapstructure:"batch"`
}

// BatchConfig bounds the batch endpoint: MaxPoints is the hard request cap,
// MaxSyncPoints the synchronous-mode cap (larger batches must stream NDJSON),
// Concurrency the batch worker-pool size.
type BatchConfig struct {
	MaxPoints     int `mapstructure:"max_points"`
	MaxSyncPoints int `mapstructure:"max_sync_points"`
	Concurrency   int `mapstructure:"concurrency"`
}

type ProvidersConfig struct {
	OpenMeteo OpenMeteoConfig `mapstructure:"openmeteo"`
	Aggregate AggregateConfig `mapstructure:"aggregate"`
	Bioclim   BioclimConfig   `mapstructure:"bioclim"`
}

// BioclimConfig configures the bioclim provider (BIO variables + Köppen-Geiger,
// computed from ERA5 monthly normals). It reuses the Open-Meteo archive URL.
type BioclimConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type OpenMeteoConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	ArchiveBaseURL  string        `mapstructure:"archive_base_url"`
	ForecastBaseURL string        `mapstructure:"forecast_base_url"`
	Timeout         time.Duration `mapstructure:"timeout"`
	ArchiveDelay    time.Duration `mapstructure:"archive_delay"`
	RatePerMinute   int           `mapstructure:"rate_per_minute"`
	RetryAttempts   int           `mapstructure:"retry_attempts"`
	DailyBudget     int           `mapstructure:"daily_budget"`
	Weights         CallWeights   `mapstructure:"weights"`
}

// CallWeights are the estimated Open-Meteo cost weights per call type; long
// archive ranges count as multiple calls upstream, so bioclim's 30-year daily
// series weighs far more than a single-day weather call.
type CallWeights struct {
	Weather   int `mapstructure:"weather"`
	Aggregate int `mapstructure:"aggregate"`
	Bioclim   int `mapstructure:"bioclim"`
}

// AggregateConfig configures the weather-aggregate provider (antecedent
// precipitation, daily temperature extrema, growing-degree-days at fixed bases
// 5 and 10 °C, and the Arrhenius thermal-time index). It reuses the Open-Meteo
// endpoints.
type AggregateConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// Defaults registers every default. Call before Load (and from cmd initConfig).
func Defaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", 30*time.Second)
	viper.SetDefault("server.shutdown_timeout", 15*time.Second)
	// Registering the key is what lets AutomaticEnv pick up
	// TEMPUS_SERVER_CORS_ALLOWED_ORIGINS; the empty default keeps CORS off.
	viper.SetDefault("server.cors.allowed_origins", []string{})
	viper.SetDefault("server.rate_limit.enabled", false)
	viper.SetDefault("server.rate_limit.rate", 100.0)
	viper.SetDefault("server.rate_limit.burst", 200)
	viper.SetDefault("server.rate_limit.trusted_proxies", []string{})

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
	viper.SetDefault("query.batch.max_points", 10000)
	viper.SetDefault("query.batch.max_sync_points", 1000)
	viper.SetDefault("query.batch.concurrency", 4)
	viper.SetDefault("providers.openmeteo.enabled", true)
	viper.SetDefault("providers.openmeteo.archive_base_url", "https://archive-api.open-meteo.com/v1/archive")
	viper.SetDefault("providers.openmeteo.forecast_base_url", "https://api.open-meteo.com/v1/forecast")
	viper.SetDefault("providers.openmeteo.timeout", 10*time.Second)
	viper.SetDefault("providers.openmeteo.archive_delay", 5*24*time.Hour)
	viper.SetDefault("providers.openmeteo.rate_per_minute", 500)
	viper.SetDefault("providers.openmeteo.retry_attempts", 3)
	viper.SetDefault("providers.openmeteo.daily_budget", 8000)
	viper.SetDefault("providers.openmeteo.weights.weather", 1)
	viper.SetDefault("providers.openmeteo.weights.aggregate", 2)
	viper.SetDefault("providers.openmeteo.weights.bioclim", 30)
	viper.SetDefault("providers.aggregate.enabled", true)
	viper.SetDefault("providers.bioclim.enabled", true)
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
