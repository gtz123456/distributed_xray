package email

import (
	"fmt"
	"net/smtp"
	"os"
)

func SendEmail(to string, subject string, body string) error {
	smtpServer := os.Getenv("SMTP_SERVER")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	fromAddr := os.Getenv("FROM_ADDR")
	fromName := os.Getenv("FROM_NAME")

	if smtpServer == "" || smtpPort == "" || smtpUser == "" || smtpPass == "" || fromAddr == "" || fromName == "" {
		return fmt.Errorf(
			"missing required SMTP environment variables: SMTP_SERVER=%s, SMTP_PORT=%s, SMTP_USER=%s, SMTP_PASS=%s, FROM_ADDR=%s, FROM_NAME=%s",
			smtpServer, smtpPort, smtpUser, smtpPass, fromAddr, fromName)
	}
	msg := []byte(fmt.Sprintf("From: %s <%s>\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n"+
		"%s",
		fromName, fromAddr, to, subject, body))

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpServer)

	err := smtp.SendMail(smtpServer+":"+smtpPort, auth, fromAddr, []string{to}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func SendVerificationEmail(to string, token string) error {
	subject := "Please verify your email address for FreewayVPN account"
	verifyLink := fmt.Sprintf("http://%s/verify?token=%s", os.Getenv("Web_Host"), token)
	body := fmt.Sprintf("Click the link below to verify your email address:\n\n%s\n\nThis link will expire in 24 hours.", verifyLink)
	return SendEmail(to, subject, body)
}
