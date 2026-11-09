package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-ini/ini"
	"github.com/pquerna/otp"
)

type RouteConfig struct {
	APIVersion       string
	EnableCORS       bool
	CORSOrigins      []string
	CORSHeaders      []string
	UploadVolumePath string
}

type SMTPconfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	FromEmail    string
	FromName     string
}
type OTPconfig struct {
	SecretKey string
	OTPExpiry time.Duration
	OTPLength otp.Digits
}
type Tokenconfig struct {
	TokenSecret        string
	Issuer             string
	TokenExpiry        time.Duration
	RefreshTokenExpiry time.Duration
}

func SMTP() SMTPconfig {
	return SMTPconfig{
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     os.Getenv("SMTP_PORT"),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		FromEmail:    os.Getenv("FROM_EMAIL"),
		FromName:     os.Getenv("FROM_NAME"),
	}
}
func OTP() OTPconfig {
	return OTPconfig{
		SecretKey: os.Getenv("SECRET_KEY"),
		OTPExpiry: GetEnvAsDuration("OTP_EXPIRY", 100),
		OTPLength: otp.Digits(GetEnvAsInt("OTP_LENGTH", 6)),
	}
}
func Token() Tokenconfig {
	return Tokenconfig{
		TokenSecret:        os.Getenv("TOKEN_SECRET"),
		TokenExpiry:        GetEnvAsDuration("TOKEN_EXPIRY", 4*3600),
		RefreshTokenExpiry: GetEnvAsDuration("REFRESH_TOKEN_EXPIRY", 120*60*60),
		Issuer:             os.Getenv("ISS"),
	}
}

type Config struct {
	UploadVolumePath string
	// …other keys…
}

func Load() *Config {
	return &Config{
		UploadVolumePath: os.Getenv("UPLOAD_VOLUME_PATH"),
	}
}

type CookieConfig struct {
	AccessToken  CookieSettings
	RefreshToken CookieSettings
}

type CookieSettings struct {
	Path     string
	Domain   string
	Secure   bool
	HTTPOnly bool
}

type BcryptCostSetting struct {
	Cost int
}

func BcryptCost() BcryptCostSetting {
	return BcryptCostSetting{
		Cost: GetEnvAsInt("COST", 3),
	}
}

func Cookie() (*CookieConfig, error) {
	cfg, err := ini.Load("config/config.ini")
	if err != nil {
		return nil, err
	}

	accessSection := cfg.Section("cookie_access_token")
	refreshSection := cfg.Section("cookie_refresh_token")

	return &CookieConfig{
		AccessToken: CookieSettings{
			Path:     accessSection.Key("path").MustString("/"),
			Domain:   accessSection.Key("domain").MustString(""),
			Secure:   accessSection.Key("secure").MustBool(false),
			HTTPOnly: accessSection.Key("http_only").MustBool(true),
		},
		RefreshToken: CookieSettings{
			Path:     refreshSection.Key("path").MustString("/auth/access"),
			Domain:   refreshSection.Key("domain").MustString(""),
			Secure:   refreshSection.Key("secure").MustBool(false),
			HTTPOnly: refreshSection.Key("http_only").MustBool(true),
		},
	}, nil
}

func GetEnvAsDuration(key string, defaultHours int) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return time.Duration(defaultHours) * time.Second
	}

	// Try parsing as duration first (e.g., "4h", "120h")
	duration, err := time.ParseDuration(val)
	if err == nil {
		return duration
	}

	// If that fails, try parsing as hours (e.g., "4", "120")
	hours, err := strconv.Atoi(val)
	if err != nil {
		fmt.Printf("Error parsing %s as duration or hours: %v\n", key, err)
		return time.Duration(defaultHours) * time.Second
	}
	return time.Duration(hours) * time.Second
}

func GetEnvAsInt(key string, defaultValue int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	res, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		fmt.Printf("Error parsing %s as int: %v\n", key, err)
		return defaultValue
	}
	return int(res)
}

func LoadConfig() (*RouteConfig, error) {
	cfg, err := ini.Load("config/config.ini")
	if err != nil {
		return nil, err
	}

	config := &RouteConfig{
		APIVersion:       cfg.Section("api").Key("version").String(),
		EnableCORS:       cfg.Section("cors").Key("enable").MustBool(false),
		UploadVolumePath: os.Getenv("STORAGE_PATH") + cfg.Section("storage").Key("upload_volume_path").MustString(os.Getenv("UPLOAD_VOLUME_PATH")),
	}
	origins := os.ExpandEnv(cfg.Section("cors").Key("origins").String())
	config.CORSOrigins = splitOrigins(origins)
	headers := cfg.Section("cors").Key("headers").String()
	config.CORSHeaders = splitOrigins(headers)
	return config, nil
}

func splitOrigins(origins string) []string {
	return strings.Split(origins, ",")
}

func GetLatestChangesRefreshInterval() time.Duration {
	cfg, err := ini.Load("config/config.ini")
	if err != nil {
		return 30 * time.Second
	}
	str := cfg.Section("storage").Key("latest_changes_refresh_interval").MustString("30s")
	d, err := time.ParseDuration(str)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// DBPool is the database/sql pool for MySQL on the VM.
// Idle matches open so a request reuses a connection.
type DBPool struct {
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PingInterval    time.Duration
	PingTimeout     time.Duration
}

func LoadDBPool() DBPool {
	pool := DBPool{
		MaxOpenConns:    50,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 4 * time.Minute,
		PingInterval:    30 * time.Second,
		PingTimeout:     8 * time.Second,
	}
	cfg, err := ini.Load("config/config.ini")
	if err == nil {
		sec := cfg.Section("db_pool")
		pool.MaxOpenConns = sec.Key("max_open_conns").MustInt(pool.MaxOpenConns)
		if raw := strings.TrimSpace(sec.Key("conn_max_lifetime").String()); raw != "" {
			if d, parseErr := time.ParseDuration(raw); parseErr == nil && d > 0 {
				pool.ConnMaxLifetime = d
			}
		}
		if raw := strings.TrimSpace(sec.Key("conn_max_idle_time").String()); raw != "" {
			if d, parseErr := time.ParseDuration(raw); parseErr == nil && d > 0 {
				pool.ConnMaxIdleTime = d
			}
		}
		if raw := strings.TrimSpace(sec.Key("ping_interval").String()); raw != "" {
			if d, parseErr := time.ParseDuration(raw); parseErr == nil && d > 0 {
				pool.PingInterval = d
			}
		}
		if raw := strings.TrimSpace(sec.Key("ping_timeout").String()); raw != "" {
			if d, parseErr := time.ParseDuration(raw); parseErr == nil && d > 0 {
				pool.PingTimeout = d
			}
		}
	}
	if v := os.Getenv("DB_MAX_OPEN_CONNS"); v != "" {
		if n, parseErr := strconv.Atoi(v); parseErr == nil && n > 0 {
			pool.MaxOpenConns = n
		}
	}
	if v := os.Getenv("DB_CONN_MAX_LIFETIME"); v != "" {
		if d, parseErr := time.ParseDuration(v); parseErr == nil && d > 0 {
			pool.ConnMaxLifetime = d
		}
	}
	return pool
}
