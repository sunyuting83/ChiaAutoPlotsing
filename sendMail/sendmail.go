package sendmail

import (
	"log"
	"net/smtp"
)

func SendEmail(receiver, SMTPHost, SMTPPort, SMTPUsername, SMTPPassword, Title, Content string) {
	auth := smtp.PlainAuth("", SMTPUsername, SMTPPassword, SMTPHost)
	msg := []byte("Subject: " + Title + "\r\n\r\n" + Content + "\r\n")
	err := smtp.SendMail(SMTPHost+SMTPPort, auth, SMTPUsername, []string{receiver}, msg)
	if err != nil {
		log.Fatal("failed to send email:", err)
	}
}
