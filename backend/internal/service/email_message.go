package service

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
)

type smtpMessage struct {
	envelopeFrom string
	envelopeTo   string
	data         []byte
}

// EmailAttachment carries a file whose bytes are preserved during MIME encoding.
type EmailAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

func buildSMTPMessage(config *SMTPConfig, to, subject, body string, attachments ...EmailAttachment) (smtpMessage, error) {
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

	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", fromHeader)
	fmt.Fprintf(&message, "To: %s\r\n", toHeader)
	fmt.Fprintf(&message, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&message, "Message-ID: %s\r\n", messageID)
	fmt.Fprintf(&message, "Subject: %s\r\n", subjectHeader)
	fmt.Fprint(&message, "MIME-Version: 1.0\r\n")
	if len(attachments) == 0 {
		fmt.Fprint(&message, "Content-Type: text/html; charset=UTF-8\r\n"+
			"Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		if err := writeSMTPHTMLBody(&message, body); err != nil {
			return smtpMessage{}, err
		}
	} else {
		writer := multipart.NewWriter(&message)
		contentType := mime.FormatMediaType("multipart/mixed", map[string]string{"boundary": writer.Boundary()})
		fmt.Fprintf(&message, "Content-Type: %s\r\n\r\n", contentType)
		header := make(textproto.MIMEHeader)
		header.Set("Content-Type", "text/html; charset=UTF-8")
		header.Set("Content-Transfer-Encoding", "quoted-printable")
		part, err := writer.CreatePart(header)
		if err != nil {
			return smtpMessage{}, fmt.Errorf("create email body part: %w", err)
		}
		if err := writeSMTPHTMLBody(part, body); err != nil {
			return smtpMessage{}, err
		}
		for _, attachment := range attachments {
			if err := writeSMTPAttachment(writer, attachment); err != nil {
				return smtpMessage{}, err
			}
		}
		if err := writer.Close(); err != nil {
			return smtpMessage{}, fmt.Errorf("close multipart email: %w", err)
		}
	}

	return smtpMessage{
		envelopeFrom: fromAddress.Address,
		envelopeTo:   recipientAddress.Address,
		data:         message.Bytes(),
	}, nil
}

func writeSMTPHTMLBody(w io.Writer, body string) error {
	bodyWriter := quotedprintable.NewWriter(w)
	if _, err := bodyWriter.Write([]byte(body)); err != nil {
		return fmt.Errorf("encode email body: %w", err)
	}
	if err := bodyWriter.Close(); err != nil {
		return fmt.Errorf("close email body encoder: %w", err)
	}
	return nil
}

func writeSMTPAttachment(writer *multipart.Writer, attachment EmailAttachment) error {
	filename := strings.TrimSpace(sanitizeEmailHeader(attachment.Filename))
	if filename == "" {
		return errors.New("email attachment filename is empty")
	}
	contentType := strings.TrimSpace(attachment.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("invalid email attachment content type: %w", err)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Type", mime.FormatMediaType(mediaType, params))
	header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	header.Set("Content-Transfer-Encoding", "base64")
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create email attachment part: %w", err)
	}
	// 57 bytes encode to one 76-character MIME line. Base64 preserves the exact
	// attachment bytes, including UTF-8 text and its original line endings.
	var encoded [76]byte
	for data := attachment.Data; len(data) > 0; {
		n := min(len(data), 57)
		base64.StdEncoding.Encode(encoded[:], data[:n])
		if _, err := part.Write(encoded[:base64.StdEncoding.EncodedLen(n)]); err != nil {
			return fmt.Errorf("encode email attachment: %w", err)
		}
		if _, err := io.WriteString(part, "\r\n"); err != nil {
			return fmt.Errorf("write email attachment line break: %w", err)
		}
		data = data[n:]
	}
	return nil
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
