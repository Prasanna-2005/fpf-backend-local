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
		UploadVolumePath: cfg.Section("storage").Key("upload_volume_path").MustString(os.Getenv("UPLOAD_VOLUME_PATH")),
	}
	origins := cfg.Section("cors").Key("origins").String()
	config.CORSOrigins = splitOrigins(origins)
	return config, nil
}

func splitOrigins(origins string) []string {
	return strings.Split(origins, ",")
}
