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
	APIVersion  string
	EnableCORS  bool
	CORSOrigins []string
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
	TokenSecret string
	TokenExpiry time.Duration
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
		OTPExpiry: GetEnvAsDuration("OTP_EXPIRY", 45*time.Second),
		OTPLength: otp.Digits(GetEnvAsInt("OTP_LENGTH", 6)),
	}
}
func Token() Tokenconfig {
	return Tokenconfig{
		TokenSecret: os.Getenv("TOKEN_SECRET"),
		TokenExpiry: GetEnvAsDuration("TOKEN_EXPIRY", 120),
	}
}

func GetEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(val)
	if err != nil {
		fmt.Printf("Error parsing %s as duration: %v\n", key, err)
		return defaultValue
	}
	return duration
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

func LoadConfig(filePath string) (*RouteConfig, error) {
	cfg, err := ini.Load("config/config.ini")
	if err != nil {
		return nil, err
	}

	config := &RouteConfig{
		APIVersion: cfg.Section("api").Key("version").String(),
		EnableCORS: cfg.Section("cors").Key("enable").MustBool(false),
	}

	origins := cfg.Section("cors").Key("origins").String()
	config.CORSOrigins = splitOrigins(origins)

	return config, nil
}

func splitOrigins(origins string) []string {
	return strings.Split(origins, ",")
}
