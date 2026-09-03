package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment         string
	StartupProbeTimeout time.Duration
	Server              ServerConfig
	Postgres            PostgresConfig
	Redis               RedisConfig
	Log                 LogConfig
	CORS                CORSConfig
	RateLimit           RateLimitConfig
	Features            FeatureConfig
}

type ServerConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
}

type PostgresConfig struct {
	URL      string
	MaxConns int32
	MinConns int32
}

type RedisConfig struct {
	URL string
}

type LogConfig struct {
	Level  string
	Format string
}

type CORSConfig struct {
	AllowedOrigins []string
}

type RateLimitConfig struct {
	Enabled           bool
	Requests          int
	Window            time.Duration
	FailOpen          bool
	TrustProxyHeaders bool
}

type FeatureConfig struct {
	DebugRoutes bool
	Docs        bool
}

func Load() (Config, error) {
	var cfg Config
	l := newLoader()

	cfg.Environment = l.enum("ENVIRONMENT", "development", "development", "staging", "production")
	cfg.StartupProbeTimeout = l.duration("STARTUP_PROBE_TIMEOUT", 5*time.Second)

	loadServerConfig(&cfg, l)
	loadPostgresConfig(&cfg, l)
	loadRedisConfig(&cfg, l)
	loadLogConfig(&cfg, l)
	loadCORS(&cfg, l)
	loadRateLimit(&cfg, l)
	loadFeatures(&cfg, l)

	err := l.err()
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Addr() string {
	return net.JoinHostPort(
		"",
		strconv.Itoa(c.Server.Port),
	)
}

func LoadEnvFile() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := wd

	for level := 0; level <= 5; level++ {
		path := filepath.Join(dir, ".env")

		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Load(path); err != nil {
				return "", err
			}

			return path, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return "", nil
}

func loadServerConfig(cfg *Config, l *loader) {
	cfg.Server = ServerConfig{
		Port:            l.integer("BACKEND_PORT", 8080),
		ReadTimeout:     l.duration("SERVER_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:    l.duration("SERVER_WRITE_TIMEOUT", 35*time.Second),
		IdleTimeout:     l.duration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		RequestTimeout:  l.duration("REQUEST_TIMEOUT", 15*time.Second),
		ShutdownTimeout: l.duration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	if cfg.Server.WriteTimeout <= cfg.Server.RequestTimeout {
		l.invalid = append(
			l.invalid,
			fmt.Sprintf(
				"SERVER_WRITE_TIMEOUT must be greater than REQUEST_TIMEOUT, so a timeout response can still be written (got SERVER_WRITE_TIMEOUT=%s, REQUEST_TIMEOUT=%s)",
				cfg.Server.WriteTimeout,
				cfg.Server.RequestTimeout,
			),
		)
	}
}

func loadPostgresConfig(cfg *Config, l *loader) {
	postgresDB := l.required("POSTGRES_DB")
	postgresUser := l.required("POSTGRES_USER")
	postgresPassword := l.required("POSTGRES_PASSWORD")
	postgresHost := l.required("POSTGRES_HOST")
	postgresPort := l.integer("POSTGRES_PORT", 5432)
	postgresSSLMode := l.enum("POSTGRES_SSLMODE", "disable", "disable", "allow", "prefer", "require", "verify-ca", "verify-full")

	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(postgresUser, postgresPassword),
		Host:     net.JoinHostPort(postgresHost, strconv.Itoa(postgresPort)),
		Path:     "/" + postgresDB,
		RawQuery: url.Values{"sslmode": {postgresSSLMode}}.Encode(),
	}

	cfg.Postgres = PostgresConfig{
		URL:      dsn.String(),
		MaxConns: l.integer32("POSTGRES_MAX_CONNS", 10),
		MinConns: l.integer32("POSTGRES_MIN_CONNS", 2),
	}

	if cfg.Postgres.MaxConns < cfg.Postgres.MinConns {
		l.invalid = append(
			l.invalid,
			fmt.Sprintf(
				"POSTGRES_MAX_CONNS must be greater than or equal to POSTGRES_MIN_CONNS (got POSTGRES_MAX_CONNS=%d, POSTGRES_MIN_CONNS=%d)",
				cfg.Postgres.MaxConns,
				cfg.Postgres.MinConns,
			),
		)
	}
}

func loadRedisConfig(cfg *Config, l *loader) {
	redisHost := l.required("REDIS_HOST")
	redisPort := l.integer("REDIS_PORT", 6379)
	redisDB := l.integer("REDIS_DB", 0)

	dsn := url.URL{
		Scheme: "redis",
		Host:   net.JoinHostPort(redisHost, strconv.Itoa(redisPort)),
		Path:   "/" + strconv.Itoa(redisDB),
	}

	cfg.Redis = RedisConfig{
		URL: dsn.String(),
	}
}

func loadLogConfig(cfg *Config, l *loader) {
	cfg.Log = LogConfig{
		Format: l.enum("LOG_FORMAT", "json", "json", "text"),
		Level:  l.enum("LOG_LEVEL", "info", "debug", "info", "warn", "error"),
	}
}

func loadCORS(cfg *Config, l *loader) {
	cfg.CORS = CORSConfig{
		AllowedOrigins: l.list("CORS_ALLOWED_ORIGINS", "http://localhost:5173"),
	}
}

func loadRateLimit(cfg *Config, l *loader) {
	cfg.RateLimit = RateLimitConfig{
		Enabled:           l.boolean("RATE_LIMIT_ENABLED", true),
		Requests:          l.integer("RATE_LIMIT_REQUESTS", 100),
		Window:            l.duration("RATE_LIMIT_WINDOW", time.Minute),
		FailOpen:          l.enum("RATE_LIMIT_FAIL_MODE", "closed", "closed", "open") == "open",
		TrustProxyHeaders: l.boolean("TRUST_PROXY_HEADERS", false),
	}
}

func loadFeatures(cfg *Config, l *loader) {
	cfg.Features = FeatureConfig{
		DebugRoutes: l.boolean("DEBUG_ROUTES_ENABLED", false),
		Docs:        l.boolean("DOCS_ENABLED", false),
	}
}
