package ctrl

import (
    "bytes"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io/ioutil"
    "net/http"
    "net/smtp"
    "os"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "path/filepath"
)

type OTPConfig struct {
    SMTPHost     string
    SMTPPort     string
    SMTPUser     string
    SMTPPassword string
    FromEmail    string
    FromName     string
    OTPLength    int
    OTPExpiry    time.Duration
}

var otpConfig OTPConfig

func init() {
    godotenv.Load()
    otpConfig = OTPConfig{
        SMTPHost:     os.Getenv("SMTP_HOST"),
        SMTPPort:     os.Getenv("SMTP_PORT"),
        SMTPUser:     os.Getenv("SMTP_USER"),
        SMTPPassword: os.Getenv("SMTP_PASSWORD"),
        FromEmail:    os.Getenv("FROM_EMAIL"),
        FromName:     os.Getenv("FROM_NAME"),
        OTPLength:    mustParseInt(os.Getenv("OTP_LENGTH")),
        OTPExpiry:    mustParseDuration(os.Getenv("OTP_EXPIRY")),
    }
}

func mustParseInt(s string) int {
    i, err := strconv.Atoi(s)
    if err != nil {
        panic(err)
    }
    return i
}

func mustParseDuration(s string) time.Duration {
    d, err := strconv.Atoi(s)
    if err != nil {
        panic(err)
    }
    return time.Duration(d) * time.Minute
}

func generateSecureOTP(length int) (string, string, error) {
    if length < 6 || length > 8 {
        return "", "", fmt.Errorf("OTP length must be between 6 and 8 digits")
    }

    const digits = "0123456789"
    otp := make([]byte, length)

    for i := range otp {
        num, err := rand.Int(rand.Reader, big.NewInt(10))
        if err != nil {
            return "", "", fmt.Errorf("failed to generate OTP digit: %w", err)
        }
        otp[i] = digits[num.Int64()]
    }

    code := string(otp)
    hash := sha256.Sum256([]byte(code))
    return code, hex.EncodeToString(hash[:]), nil
}

func storeOTPInEnv(otp, hash string) error {
    envFilePath := filepath.Join("form_pf_backend", ".env")

    // Read existing content of .env file
    data, err := ioutil.ReadFile(envFilePath)
    if err != nil {
        return fmt.Errorf("failed to read .env file: %w", err)
    }

    // Append OTP and hash to .env file
    content := fmt.Sprintf("\nOTP=%s\nOTP_HASH=%s\n", otp, hash)
    data = append(data, []byte(content)...)

    // Write updated content back to .env file
    err = ioutil.WriteFile(envFilePath, data, 0644)
    if err !=