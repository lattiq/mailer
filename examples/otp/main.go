package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/lattiq/mailer"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	config := mailer.DefaultConfig()
	config.Templates.Enabled = true
	config.Templates.Directory = "templates" // Point to our templates directory
	config.Templates.Extension = []string{".html", ".text"}

	// For local testing without AWS credentials, swap in:
	//   mailer.WithDryRun(mailer.DryRunOptions{})
	client, err := mailer.New(config, mailer.WithAWSSES("ap-south-1"))
	if err != nil {
		return err
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			log.Printf("client close: %v", cerr)
		}
	}()

	otp, err := generateOTP()
	if err != nil {
		return fmt.Errorf("generate OTP: %w", err)
	}

	otpData := OTPData{
		UserName:       "John Doe",
		OTP:            otp,
		ExpiryTime:     getOTPExpiryTime(10), // 10 minutes expiry
		ExpiryDuration: fmt.Sprintf("%d minutes", 10),
		CompanyName:    "LattIQ",
		CompanyLogo:    "https://i.postimg.cc/Mp3s4bHn/lattiq-logo-black.png",
		SupportEmail:   "support@lattiq.com",
		AppName:        "LattIQ Hub",
	}

	templateRequest := &mailer.TemplateRequest{
		Template: "otp", // This will use otp.html and otp.text templates
		From:     mailer.Address{Email: "otp@lattiq.com", Name: "LattIQ"},
		To:       []mailer.Address{{Email: "infra@lattiq.com", Name: "LattIQ Infra Team"}},
		Subject:  fmt.Sprintf("Your %s login code: %s", otpData.AppName, otpData.OTP),
		Data:     otpData,
		Headers: map[string]string{
			"X-Category": "authentication",
			"X-OTP-Type": "login",
		},
	}

	if err := client.SendTemplate(context.Background(), templateRequest); err != nil {
		return fmt.Errorf("send template-based OTP email: %w", err)
	}

	log.Println("Email sent successfully using templates!")
	return nil
}

// OTPData represents the data structure for OTP email templates
type OTPData struct {
	UserName       string
	OTP            string
	ExpiryTime     string
	ExpiryDuration string
	CompanyName    string
	CompanyLogo    string
	SupportEmail   string
	AppName        string
}

// generateOTP creates a cryptographically secure random 6-digit OTP
func generateOTP() (string, error) {
	// Generate cryptographically secure random number
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// getOTPExpiryTime returns the expiry time string for OTP
func getOTPExpiryTime(minutes int) string {
	return time.Now().Add(time.Duration(minutes) * time.Minute).Format("3:04 PM MST")
}
