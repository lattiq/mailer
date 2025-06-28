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
	config := mailer.DefaultConfig()
	config.Templates.Enabled = true
	config.Templates.Directory = "templates" // Point to our templates directory
	config.Templates.Extension = []string{".html", ".text"}

	client, err := mailer.New(config, mailer.WithAWSSES("ap-south-1"))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Prepare OTP data
	otp, err := generateOTP()
	if err != nil {
		log.Fatal("Failed to generate OTP:", err)
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

	// Create template request instead of direct email
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

	// Send using template
	if err := client.SendTemplate(context.Background(), templateRequest); err != nil {
		log.Fatal("Failed to send template-based OTP email:", err)
	}

	fmt.Printf("✅ Template-based OTP email sent successfully!\n")
	log.Println("Email sent successfully using templates!")
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
