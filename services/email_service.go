// File: /services/email_service.go
package services

import (
	"crypto/rand"
	"fmt"
	"gopkg.in/gomail.v2"
	"math/big"
	"motocosmos-api/config"
	"sync"
	"time"
)

type EmailService struct {
	config *config.Config
	dialer *gomail.Dialer

	// In-memory storage for verification codes
	verificationCodes map[string]VerificationCode
	mutex             sync.RWMutex
}

type VerificationCode struct {
	Code      string    `json:"code"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
}

func NewEmailService(cfg *config.Config) *EmailService {
	dialer := gomail.NewDialer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword)

	service := &EmailService{
		config:            cfg,
		dialer:            dialer,
		verificationCodes: make(map[string]VerificationCode),
	}

	// Start cleanup goroutine
	go service.cleanupExpiredCodes()

	return service
}

// Generate a random 4-digit verification code
func (es *EmailService) generateVerificationCode() string {
	const digits = "0123456789"
	code := make([]byte, 4)

	for i := range code {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		code[i] = digits[num.Int64()]
	}

	return string(code)
}

// Send verification email
func (es *EmailService) SendVerificationEmail(email, name string) (string, error) {
	// Check if there's already a valid unused code
	es.mutex.RLock()
	existingCode, exists := es.verificationCodes[email]
	es.mutex.RUnlock()

	var code string
	if exists && !existingCode.Used && time.Now().Before(existingCode.ExpiresAt) {
		// Reuse existing valid code
		code = existingCode.Code
		fmt.Printf("📧 Reusing existing verification code for %s: %s\n", email, code)
	} else {
		// Generate new code
		code = es.generateVerificationCode()

		// Store verification code (expires in 10 minutes)
		es.mutex.Lock()
		es.verificationCodes[email] = VerificationCode{
			Code:      code,
			Email:     email,
			ExpiresAt: time.Now().Add(10 * time.Minute),
			Used:      false,
		}
		es.mutex.Unlock()
		fmt.Printf("📧 Generated new verification code for %s: %s\n", email, code)
	}

	// Create email message
	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", es.config.FromName, es.config.FromEmail))
	m.SetHeader("To", email)
	m.SetHeader("Subject", "MotoCosmos - Email Verification")

	// HTML email template
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Email Verification</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { text-align: center; background: #007bff; color: white; padding: 20px; border-radius: 10px 10px 0 0; }
        .content { background: #f8f9fa; padding: 30px; border-radius: 0 0 10px 10px; }
        .code { background: #e9ecef; padding: 20px; text-align: center; border-radius: 8px; margin: 20px 0; }
        .code-number { font-size: 32px; font-weight: bold; color: #007bff; letter-spacing: 8px; }
        .footer { text-align: center; margin-top: 20px; color: #666; font-size: 14px; }
        .btn { display: inline-block; background: #007bff; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏍️ MotoCosmos</h1>
            <p>Email Verification</p>
        </div>
        <div class="content">
            <h2>Hello %s!</h2>
            <p>Welcome to MotoCosmos! Please verify your email address to complete your registration.</p>
            
            <div class="code">
                <p><strong>Your verification code is:</strong></p>
                <div class="code-number">%s</div>
                <p><small>This code will expire in 10 minutes.</small></p>
            </div>
            
            <p>Enter this code in the MotoCosmos app to verify your email address.</p>
            
            <p>If you didn't create an account with MotoCosmos, please ignore this email.</p>
            
            <p>Happy riding! 🏍️</p>
            <p><strong>The MotoCosmos Team</strong></p>
        </div>
        <div class="footer">
            <p>© 2025 MotoCosmos. All rights reserved.</p>
            <p>This is an automated email, please do not reply.</p>
        </div>
    </div>
</body>
</html>`, name, code)

	// Plain text alternative
	textBody := fmt.Sprintf(`
Hello %s!

Welcome to MotoCosmos! Please verify your email address to complete your registration.

Your verification code is: %s

This code will expire in 10 minutes.

Enter this code in the MotoCosmos app to verify your email address.

If you didn't create an account with MotoCosmos, please ignore this email.

Happy riding!
The MotoCosmos Team

© 2025 MotoCosmos. All rights reserved.
This is an automated email, please do not reply.
    `, name, code)

	m.SetBody("text/plain", textBody)
	m.AddAlternative("text/html", htmlBody)

	// Send email
	if err := es.dialer.DialAndSend(m); err != nil {
		return "", fmt.Errorf("failed to send email: %w", err)
	}

	fmt.Printf("📧 Verification email sent to %s with code: %s\n", email, code)
	return code, nil
}

// Verify the code
func (es *EmailService) VerifyCode(email, inputCode string) bool {
	es.mutex.RLock()
	storedCode, exists := es.verificationCodes[email]
	es.mutex.RUnlock()

	if !exists {
		fmt.Printf("❌ No verification code found for email: %s\n", email)
		return false
	}

	if storedCode.Used {
		fmt.Printf("❌ Verification code already used for: %s\n", email)
		return false
	}

	if time.Now().After(storedCode.ExpiresAt) {
		fmt.Printf("❌ Verification code expired for: %s\n", email)
		es.mutex.Lock()
		delete(es.verificationCodes, email)
		es.mutex.Unlock()
		return false
	}

	if storedCode.Code != inputCode {
		fmt.Printf("❌ Invalid verification code for %s. Expected: %s, Got: %s\n", email, storedCode.Code, inputCode)
		return false
	}

	// Mark as used
	es.mutex.Lock()
	storedCode.Used = true
	es.verificationCodes[email] = storedCode
	es.mutex.Unlock()

	fmt.Printf("✅ Verification code verified successfully for: %s\n", email)
	return true
}

// Get verification code for testing/debugging
func (es *EmailService) GetVerificationCode(email string) string {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	if code, exists := es.verificationCodes[email]; exists && !code.Used && time.Now().Before(code.ExpiresAt) {
		return code.Code
	}
	return ""
}

// Cleanup expired verification codes
func (es *EmailService) cleanupExpiredCodes() {
	ticker := time.NewTicker(5 * time.Minute) // Run every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			es.mutex.Lock()
			now := time.Now()
			for email, code := range es.verificationCodes {
				if now.After(code.ExpiresAt) || code.Used {
					delete(es.verificationCodes, email)
					fmt.Printf("🗑️ Cleaned up verification code for: %s\n", email)
				}
			}
			es.mutex.Unlock()
		}
	}
}

// Send welcome email after successful verification
func (es *EmailService) SendWelcomeEmail(email, name string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", es.config.FromName, es.config.FromEmail))
	m.SetHeader("To", email)
	m.SetHeader("Subject", "Welcome to MotoCosmos! 🏍️")

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome to MotoCosmos</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { text-align: center; background: linear-gradient(135deg, #007bff, #0056b3); color: white; padding: 30px; border-radius: 10px 10px 0 0; }
        .content { background: #f8f9fa; padding: 30px; border-radius: 0 0 10px 10px; }
        .feature { background: white; padding: 20px; margin: 15px 0; border-radius: 8px; border-left: 4px solid #007bff; }
        .footer { text-align: center; margin-top: 20px; color: #666; font-size: 14px; }
        .btn { display: inline-block; background: #007bff; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏍️ Welcome to MotoCosmos!</h1>
            <p>Your motorcycle adventure starts here</p>
        </div>
        <div class="content">
            <h2>Hello %s!</h2>
            <p>🎉 Congratulations! Your email has been verified and your MotoCosmos account is now active.</p>
            
            <h3>What you can do now:</h3>
            
            <div class="feature">
                <h4>🗺️ Plan Amazing Routes</h4>
                <p>Create and share spectacular motorcycle routes with our advanced route planning tools.</p>
            </div>
            
            <div class="feature">
                <h4>🎯 Join Community Events</h4>
                <p>Discover and participate in exciting motorcycle events in your area.</p>
            </div>
            
            <div class="feature">
                <h4>📊 Track Your Rides</h4>
                <p>Record your rides, track statistics, and share your adventures with fellow riders.</p>
            </div>
            
            <div class="feature">
                <h4>👥 Connect with Riders</h4>
                <p>Follow other motorcyclists, share experiences, and build your riding community.</p>
            </div>
            
            <p>Ready to start your first adventure? Open the MotoCosmos app and explore all the amazing features waiting for you!</p>
            
            <p>Safe rides and see you on the road! 🛣️</p>
            <p><strong>The MotoCosmos Team</strong></p>
        </div>
        <div class="footer">
            <p>© 2025 MotoCosmos. All rights reserved.</p>
            <p>Happy riding! 🏍️</p>
        </div>
    </div>
</body>
</html>`, name)

	textBody := fmt.Sprintf(`
Hello %s!

🎉 Congratulations! Your email has been verified and your MotoCosmos account is now active.

What you can do now:

🗺️ Plan Amazing Routes
Create and share spectacular motorcycle routes with our advanced route planning tools.

🎯 Join Community Events  
Discover and participate in exciting motorcycle events in your area.

📊 Track Your Rides
Record your rides, track statistics, and share your adventures with fellow riders.

👥 Connect with Riders
Follow other motorcyclists, share experiences, and build your riding community.

Ready to start your first adventure? Open the MotoCosmos app and explore all the amazing features waiting for you!

Safe rides and see you on the road!
The MotoCosmos Team

© 2025 MotoCosmos. All rights reserved.
Happy riding! 🏍️
    `, name)

	m.SetBody("text/plain", textBody)
	m.AddAlternative("text/html", htmlBody)

	if err := es.dialer.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send welcome email: %w", err)
	}

	fmt.Printf("📧 Welcome email sent to %s\n", email)
	return nil
}

// =====================================================
// PASSWORD RESET METHODS
// =====================================================

// SendPasswordResetEmail sends a password reset verification code
func (es *EmailService) SendPasswordResetEmail(email, name string) (string, error) {
	// Check if there's already a valid unused code
	es.mutex.RLock()
	existingCode, exists := es.verificationCodes[email]
	es.mutex.RUnlock()

	var code string
	if exists && !existingCode.Used && time.Now().Before(existingCode.ExpiresAt) {
		// Reuse existing valid code
		code = existingCode.Code
		fmt.Printf("🔐 Reusing existing password reset code for %s: %s\n", email, code)
	} else {
		// Generate new code
		code = es.generateVerificationCode()

		// Store verification code (expires in 10 minutes)
		es.mutex.Lock()
		es.verificationCodes[email] = VerificationCode{
			Code:      code,
			Email:     email,
			ExpiresAt: time.Now().Add(10 * time.Minute),
			Used:      false,
		}
		es.mutex.Unlock()
		fmt.Printf("🔐 Generated new password reset code for %s: %s\n", email, code)
	}

	// Create email message
	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", es.config.FromName, es.config.FromEmail))
	m.SetHeader("To", email)
	m.SetHeader("Subject", "Password Reset - MotoCosmos")

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .code-box { background: white; border: 2px dashed #667eea; padding: 20px; text-align: center; margin: 20px 0; border-radius: 8px; }
        .code { font-size: 32px; font-weight: bold; color: #667eea; letter-spacing: 8px; }
        .warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔐 Password Reset Request</h1>
        </div>
        <div class="content">
            <p>Hi %s,</p>
            <p>We received a request to reset your password for your MotoCosmos account.</p>
            
            <div class="code-box">
                <p style="margin: 0; color: #666;">Your verification code is:</p>
                <div class="code">%s</div>
                <p style="margin: 10px 0 0 0; color: #666; font-size: 14px;">This code will expire in 10 minutes</p>
            </div>

            <div class="warning">
                <strong>⚠️ Security Notice:</strong><br>
                If you didn't request a password reset, please ignore this email. Your password will remain unchanged.
            </div>

            <p>For security reasons, never share this code with anyone.</p>
        </div>
        <div class="footer">
            <p>© 2024 MotoCosmos. All rights reserved.</p>
            <p>This is an automated message, please do not reply.</p>
        </div>
    </div>
</body>
</html>
`, name, code)

	textBody := fmt.Sprintf(`
Hi %s!

We received a request to reset your password for your MotoCosmos account.

Your verification code is: %s

This code will expire in 10 minutes.

⚠️ Security Notice:
If you didn't request a password reset, please ignore this email. Your password will remain unchanged.

For security reasons, never share this code with anyone.

The MotoCosmos Team

© 2024 MotoCosmos. All rights reserved.
This is an automated message, please do not reply.
    `, name, code)

	m.SetBody("text/plain", textBody)
	m.AddAlternative("text/html", htmlBody)

	// Send email
	if err := es.dialer.DialAndSend(m); err != nil {
		return "", fmt.Errorf("failed to send password reset email: %w", err)
	}

	fmt.Printf("🔐 Password reset email sent to %s with code: %s\n", email, code)
	return code, nil
}

// SendPasswordChangedEmail sends a confirmation after password is changed
func (es *EmailService) SendPasswordChangedEmail(email, name string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", es.config.FromName, es.config.FromEmail))
	m.SetHeader("To", email)
	m.SetHeader("Subject", "Password Changed Successfully - MotoCosmos")

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #28a745 0%%, #20c997 100%%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .success-box { background: #d4edda; border-left: 4px solid #28a745; padding: 15px; margin: 20px 0; border-radius: 4px; }
        .warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>✅ Password Changed</h1>
        </div>
        <div class="content">
            <p>Hi %s,</p>
            
            <div class="success-box">
                <strong>✓ Success!</strong><br>
                Your password has been changed successfully.
            </div>

            <p>You can now log in to your MotoCosmos account using your new password.</p>

            <div class="warning">
                <strong>⚠️ Didn't make this change?</strong><br>
                If you didn't change your password, please contact our support team immediately at support@motocosmos.com
            </div>

            <p>Thank you for keeping your account secure!</p>
        </div>
        <div class="footer">
            <p>© 2024 MotoCosmos. All rights reserved.</p>
            <p>This is an automated message, please do not reply.</p>
        </div>
    </div>
</body>
</html>
`, name)

	textBody := fmt.Sprintf(`
Hi %s!

✓ Success! Your password has been changed successfully.

You can now log in to your MotoCosmos account using your new password.

⚠️ Didn't make this change?
If you didn't change your password, please contact our support team immediately at support@motocosmos.com

Thank you for keeping your account secure!

The MotoCosmos Team

© 2024 MotoCosmos. All rights reserved.
This is an automated message, please do not reply.
    `, name)

	m.SetBody("text/plain", textBody)
	m.AddAlternative("text/html", htmlBody)

	if err := es.dialer.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send password changed email: %w", err)
	}

	fmt.Printf("✅ Password changed confirmation email sent to %s\n", email)
	return nil
}