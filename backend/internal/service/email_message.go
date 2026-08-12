package service

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var (
	emailStyleBlockPattern  = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)
	emailScriptBlockPattern = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	emailBreakPattern       = regexp.MustCompile(`(?i)<(?:br\s*/?|/p|/div|/h[1-6]|/li)>`)
	emailTagPattern         = regexp.MustCompile(`(?s)<[^>]+>`)
	emailBlankLinesPattern  = regexp.MustCompile(`\n{3,}`)
)

type smtpMessage struct {
	envelopeFrom string
	envelopeTo   string
	data         []byte
}

func buildSMTPMessage(config *SMTPConfig, to, subject, body string) (smtpMessage, error) {
	if config == nil {
		return smtpMessage{}, errors.New("missing SMTP configuration")
	}

	fromAddress, err := parseSMTPAddress(config.From, "from")
	if err != nil {
		return smtpMessage{}, err
	}
	recipientAddress, err := parseSMTPAddress(to, "recipient")
	if err != nil {
		return smtpMessage{}, err
	}
	messageID, err := generateEmailMessageID(fromAddress.Address, config.Host)
	if err != nil {
		return smtpMessage{}, fmt.Errorf("generate message ID: %w", err)
	}

	fromName := sanitizeEmailHeader(config.FromName)
	if strings.TrimSpace(fromName) == "" {
		fromName = fromAddress.Name
	}
	fromHeader := (&mail.Address{
		Name:    fromName,
		Address: fromAddress.Address,
	}).String()
	toHeader := (&mail.Address{
		Name:    recipientAddress.Name,
		Address: recipientAddress.Address,
	}).String()
	subjectHeader := mime.QEncoding.Encode("UTF-8", sanitizeEmailHeader(subject))
	boundaryBytes := make([]byte, 18)
	if _, err := rand.Read(boundaryBytes); err != nil {
		return smtpMessage{}, fmt.Errorf("generate MIME boundary: %w", err)
	}
	boundary := "sub2api-" + hex.EncodeToString(boundaryBytes)
	plainBody := emailHTMLToPlainText(body)

	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", fromHeader)
	fmt.Fprintf(&message, "To: %s\r\n", toHeader)
	fmt.Fprintf(&message, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&message, "Message-ID: %s\r\n", messageID)
	fmt.Fprintf(&message, "Subject: %s\r\n", subjectHeader)
	fmt.Fprintf(&message, "MIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
	if err := writeEmailMIMEPart(&message, boundary, "text/plain", plainBody); err != nil {
		return smtpMessage{}, err
	}
	if err := writeEmailMIMEPart(&message, boundary, "text/html", body); err != nil {
		return smtpMessage{}, err
	}
	fmt.Fprintf(&message, "\r\n--%s--\r\n", boundary)

	return smtpMessage{
		envelopeFrom: fromAddress.Address,
		envelopeTo:   recipientAddress.Address,
		data:         message.Bytes(),
	}, nil
}

func writeEmailMIMEPart(message *bytes.Buffer, boundary, contentType, body string) error {
	fmt.Fprintf(message, "--%s\r\nContent-Type: %s; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary, contentType)
	writer := quotedprintable.NewWriter(message)
	if _, err := writer.Write([]byte(body)); err != nil {
		return fmt.Errorf("encode %s email body: %w", contentType, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close %s email body encoder: %w", contentType, err)
	}
	fmt.Fprint(message, "\r\n")
	return nil
}

func emailHTMLToPlainText(body string) string {
	plain := emailStyleBlockPattern.ReplaceAllString(body, "")
	plain = emailScriptBlockPattern.ReplaceAllString(plain, "")
	plain = emailBreakPattern.ReplaceAllString(plain, "\n")
	plain = emailTagPattern.ReplaceAllString(plain, "")
	plain = html.UnescapeString(plain)
	plain = strings.ReplaceAll(plain, "\r", "")
	lines := strings.Split(plain, "\n")
	for index := range lines {
		lines[index] = strings.TrimSpace(lines[index])
	}
	plain = strings.Join(lines, "\n")
	return strings.TrimSpace(emailBlankLinesPattern.ReplaceAllString(plain, "\n\n"))
}

func parseSMTPAddress(value, field string) (*mail.Address, error) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, fmt.Errorf("invalid SMTP %s address: contains a line break", field)
	}

	cleaned := strings.TrimSpace(value)
	address, err := mail.ParseAddress(cleaned)
	if err != nil || strings.TrimSpace(address.Address) == "" {
		if err == nil {
			err = fmt.Errorf("address is empty")
		}
		return nil, fmt.Errorf("invalid SMTP %s address: %w", field, err)
	}
	return address, nil
}

func generateEmailMessageID(fromAddress, smtpHost string) (string, error) {
	randomID := make([]byte, 16)
	if _, err := rand.Read(randomID); err != nil {
		return "", err
	}

	domain := strings.TrimSpace(sanitizeEmailHeader(smtpHost))
	if at := strings.LastIndexByte(fromAddress, '@'); at >= 0 && at < len(fromAddress)-1 {
		domain = fromAddress[at+1:]
	}
	domain = strings.Trim(domain, "[]<>")
	if domain == "" {
		domain = "localhost"
	}

	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(randomID), domain), nil
}
