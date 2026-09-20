package services

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"strings"

	"go.uber.org/zap"
	"zuri/shared/pkg/logger"
)

// EmailService handles SMTP email sending
type EmailService struct {
	host     string
	port     string
	username string
	password string
	from     string
	enabled  bool
}

// NewEmailService creates a new email service
func NewEmailService() *EmailService {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")

	if host == "" || port == "" {
		logger.Warn("SMTP configuration incomplete, email service disabled")
		return &EmailService{enabled: false}
	}

	if from == "" {
		from = "notifications@zuri.app"
	}

	logger.Info("Email service initialized",
		zap.String("smtp_host", host),
		zap.String("smtp_port", port),
	)
	return &EmailService{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
		enabled:  true,
	}
}

// SendEmail sends a plain text email
func (s *EmailService) SendEmail(to, subject, body string) error {
	if !s.enabled {
		logger.Warn("Email service is disabled, email not sent")
		return nil
	}

	msg := []byte(fmt.Sprintf(
		"To: %s\r\n"+
			"From: %s\r\n"+
			"Subject: %s\r\n"+
			"Content-Type: text/plain; charset=UTF-8\r\n"+
			"\r\n"+
			"%s",
		to, s.from, subject, body,
	))

	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	var auth smtp.Auth
	if s.username != "" && s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	err := smtp.SendMail(addr, auth, s.from, []string{to}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	logger.Debug("Email sent", zap.String("to", to))
	return nil
}

// SendHTMLEmail sends an HTML email
func (s *EmailService) SendHTMLEmail(to, subject, htmlBody string) error {
	if !s.enabled {
		logger.Warn("Email service is disabled, email not sent")
		return nil
	}

	// Create multipart message
	boundary := "boundary-zuri-" + generateBoundary()

	var msg bytes.Buffer
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "From: %s\r\n", s.from)
	fmt.Fprintf(&msg, "Subject: %s\r\n", subject)
	fmt.Fprintf(&msg, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&msg, "Content-Type: multipart/alternative; boundary=%s\r\n", boundary)
	fmt.Fprintf(&msg, "\r\n")
	fmt.Fprintf(&msg, "--%s\r\n", boundary)
	fmt.Fprintf(&msg, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&msg, "\r\n")
	fmt.Fprintf(&msg, "%s\r\n", stripHTML(htmlBody))
	fmt.Fprintf(&msg, "\r\n")
	fmt.Fprintf(&msg, "--%s\r\n", boundary)
	fmt.Fprintf(&msg, "Content-Type: text/html; charset=UTF-8\r\n")
	fmt.Fprintf(&msg, "\r\n")
	fmt.Fprintf(&msg, "%s\r\n", htmlBody)
	fmt.Fprintf(&msg, "\r\n")
	fmt.Fprintf(&msg, "--%s--\r\n", boundary)

	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	var auth smtp.Auth
	if s.username != "" && s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	err := smtp.SendMail(addr, auth, s.from, []string{to}, msg.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send HTML email: %w", err)
	}

	logger.Debug("HTML email sent", zap.String("to", to))
	return nil
}

// SendTemplateEmail sends an email using a template
func (s *EmailService) SendTemplateEmail(to, subject, templateName string, data interface{}) error {
	if !s.enabled {
		logger.Warn("Email service is disabled, email not sent")
		return nil
	}

	tmpl, ok := emailTemplates[templateName]
	if !ok {
		return fmt.Errorf("template %s not found", templateName)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return s.SendHTMLEmail(to, subject, buf.String())
}

// IsEnabled returns whether email service is enabled
func (s *EmailService) IsEnabled() bool {
	return s.enabled
}

// Email templates
var emailTemplates = map[string]*template.Template{}

func init() {
	// Quiz completion template
	emailTemplates["quiz_completed"] = template.Must(template.New("quiz_completed").Parse(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #4CAF50; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background: #f9f9f9; }
        .score { font-size: 24px; font-weight: bold; color: #4CAF50; }
        .footer { padding: 20px; text-align: center; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Quiz Completed!</h1>
        </div>
        <div class="content">
            <p>Hi {{.Name}},</p>
            <p>Congratulations on completing <strong>{{.QuizTitle}}</strong>!</p>
            <p class="score">Your Score: {{.Score}}%</p>
            <p>{{.Message}}</p>
            <p><a href="{{.QuizURL}}" style="background: #4CAF50; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">View Results</a></p>
        </div>
        <div class="footer">
            <p>You're receiving this because you completed a quiz on Zuri.</p>
        </div>
    </div>
</body>
</html>
`))

	// Streak achievement template
	emailTemplates["streak_achieved"] = template.Must(template.New("streak_achieved").Parse(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #FF9800; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background: #f9f9f9; }
        .streak { font-size: 48px; text-align: center; }
        .footer { padding: 20px; text-align: center; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔥 Streak Achieved!</h1>
        </div>
        <div class="content">
            <p>Hi {{.Name}},</p>
            <div class="streak">{{.StreakCount}} {{if eq .StreakCount 1}}day{{else}}days{{end}}</div>
            <p style="text-align: center;">You're on fire! Keep up the amazing work!</p>
            <p>{{.Message}}</p>
        </div>
        <div class="footer">
            <p>Keep the momentum going!</p>
        </div>
    </div>
</body>
</html>
`))

	// Email verification template
	emailTemplates["email_verification"] = template.Must(template.New("email_verification").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Your Zuri Verification Code</title>
    <style>
        @import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');
        
        body { 
            margin: 0; 
            padding: 0; 
            min-width: 100%; 
            width: 100% !important; 
            background-color: #fafafa; 
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
            -webkit-font-smoothing: antialiased; 
            color: #1a1a1a;
        }
        .wrapper { 
            width: 100%; 
            table-layout: fixed; 
            background-color: #fafafa; 
            padding: 64px 0; 
        }
        .container { 
            max-width: 480px; 
            margin: 0 auto; 
            background-color: #ffffff; 
            border-radius: 12px; 
            border: 1px solid #e5e7eb; 
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -1px rgba(0, 0, 0, 0.03);
            overflow: hidden;
        }
        .logo-area {
            padding: 40px 40px 0 40px;
            text-align: left;
        }
        .logo-text {
            font-size: 20px;
            font-weight: 700;
            color: #0f172a;
            letter-spacing: -0.5px;
            margin: 0;
            display: inline-flex;
            align-items: center;
        }
        .logo-dot {
            width: 6px;
            height: 6px;
            background-color: #10b981;
            border-radius: 50%;
            display: inline-block;
            margin-left: 4px;
        }
        .content { 
            padding: 32px 40px 40px 40px; 
        }
        .title {
            font-size: 22px;
            font-weight: 600;
            color: #111827;
            margin-top: 0;
            margin-bottom: 20px;
            letter-spacing: -0.4px;
        }
        .greeting { 
            font-size: 15px; 
            font-weight: 500; 
            color: #111827; 
            margin-bottom: 12px; 
        }
        .text { 
            font-size: 15px; 
            line-height: 1.6; 
            color: #4b5563; 
            margin: 0 0 28px 0; 
        }
        .code-container {
            text-align: center;
            margin: 28px 0;
        }
        .code-box { 
            background-color: #f9fafb; 
            border: 1px solid #e5e7eb;
            border-radius: 8px; 
            padding: 16px 28px; 
            display: inline-block;
        }
        .code-value { 
            font-size: 32px; 
            font-weight: 700; 
            color: #111827; 
            letter-spacing: 6px; 
            margin: 0; 
            font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace; 
            padding-left: 6px;
        }
        .expire-text { 
            font-size: 13px; 
            color: #6b7280; 
            text-align: center;
            margin-top: 16px;
            margin-bottom: 28px;
        }
        .divider { 
            height: 1px; 
            background-color: #f3f4f6; 
            margin: 28px 0; 
        }
        .security-note { 
            font-size: 13px; 
            color: #9ca3af; 
            line-height: 1.5; 
            margin: 0; 
        }
        .footer { 
            padding: 0 40px 40px 40px; 
            text-align: left; 
        }
        .footer-text { 
            font-size: 12px; 
            color: #9ca3af; 
            margin: 0; 
            line-height: 1.5;
        }
    </style>
</head>
<body>
    <div class="wrapper">
        <div class="container">
            <div class="logo-area">
                <div class="logo-text">Zuri<span class="logo-dot"></span></div>
            </div>
            <div class="content">
                <h2 class="title">Verify your email address</h2>
                <div class="greeting">Hello,</div>
                <div class="text">
                    Use the verification code below to complete your sign-in or account verification for Zuri.
                </div>
                <div class="code-container">
                    <div class="code-box">
                        <div class="code-value">{{.Code}}</div>
                    </div>
                </div>
                <div class="expire-text">This verification code will expire in 10 minutes.</div>
                <div class="divider"></div>
                <p class="security-note">
                    If you did not request this code, you can safely ignore this email.
                </p>
            </div>
            <div class="footer">
                <p class="footer-text">&copy; 2026 Zuri. All rights reserved.</p>
            </div>
        </div>
    </div>
</body>
</html>
`))

	// Password reset template
	emailTemplates["password_reset"] = template.Must(template.New("password_reset").Parse(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #F44336; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background: #f9f9f9; }
        .code { font-size: 32px; font-weight: bold; color: #F44336; text-align: center; letter-spacing: 4px; }
        .footer { padding: 20px; text-align: center; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Password Reset Request</h1>
        </div>
        <div class="content">
            <p>Hi {{.Name}},</p>
            <p>We received a request to reset your Zuri password. Use the code below:</p>
            <p class="code">{{.Code}}</p>
            <p>This code expires in 15 minutes. If you didn't request this, you can safely ignore this email.</p>
        </div>
        <div class="footer">
            <p>You're receiving this because a password reset was requested on Zuri.</p>
        </div>
    </div>
</body>
</html>
`))

	// Study reminder template
	emailTemplates["study_reminder"] = template.Must(template.New("study_reminder").Parse(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #2196F3; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background: #f9f9f9; }
        .footer { padding: 20px; text-align: center; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📚 Time to Study!</h1>
        </div>
        <div class="content">
            <p>Hi {{.Name}},</p>
            <p>{{.Message}}</p>
            <p><a href="{{.StudyURL}}" style="background: #2196F3; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">Start Studying</a></p>
        </div>
        <div class="footer">
            <p>Consistency is key to learning!</p>
        </div>
    </div>
</body>
</html>
`))
}

func generateBoundary() string {
	// Simple boundary generator
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, 16)
	for i := range result {
		result[i] = chars[i%len(chars)]
	}
	return string(result)
}

func stripHTML(html string) string {
	// Simple HTML stripper
	result := html
	result = strings.ReplaceAll(result, "<br>", "\n")
	result = strings.ReplaceAll(result, "<br/>", "\n")
	result = strings.ReplaceAll(result, "<p>", "\n")
	result = strings.ReplaceAll(result, "</p>", "")

	// Remove all tags
	for {
		start := strings.Index(result, "<")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+1:]
	}

	return strings.TrimSpace(result)
}
