//go:build unit

package service

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSMTPMessageProducesStandardsCompliantMIME(t *testing.T) {
	config := &SMTPConfig{
		Host:     "smtp.example.com",
		From:     "reply@example.com",
		FromName: "Sub2API 通知",
	}
	body := "<html>\n<body>验证码：123456 &amp; ready</body>\n</html>"

	message, err := buildSMTPMessage(config, "User <user@example.net>", "邮箱验证码", body)
	require.NoError(t, err)
	require.Equal(t, "reply@example.com", message.envelopeFrom)
	require.Equal(t, "user@example.net", message.envelopeTo)

	parsed, err := mail.ReadMessage(bytes.NewReader(message.data))
	require.NoError(t, err)

	from, err := mail.ParseAddress(parsed.Header.Get("From"))
	require.NoError(t, err)
	require.Equal(t, "Sub2API 通知", from.Name)
	require.Equal(t, "reply@example.com", from.Address)

	recipient, err := mail.ParseAddress(parsed.Header.Get("To"))
	require.NoError(t, err)
	require.Equal(t, "User", recipient.Name)
	require.Equal(t, "user@example.net", recipient.Address)

	decodedSubject, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	require.NoError(t, err)
	require.Equal(t, "邮箱验证码", decodedSubject)
	require.NotEmpty(t, parsed.Header.Get("Date"))
	_, err = mail.ParseDate(parsed.Header.Get("Date"))
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^<[0-9a-f]{32}@example\.com>$`), parsed.Header.Get("Message-ID"))
	require.Equal(t, "1.0", parsed.Header.Get("MIME-Version"))
	require.Equal(t, "quoted-printable", parsed.Header.Get("Content-Transfer-Encoding"))

	mediaType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "text/html", mediaType)
	require.Equal(t, "UTF-8", params["charset"])

	decodedBody, err := io.ReadAll(quotedprintable.NewReader(parsed.Body))
	require.NoError(t, err)
	require.Equal(t, strings.ReplaceAll(body, "\n", "\r\n"), string(decodedBody))
}

func TestBuildSMTPMessageWithAttachments(t *testing.T) {
	config := &SMTPConfig{Host: "smtp.example.com", From: "reply@example.com"}
	body := "<p>用户信息\n附件见下方。</p>"
	attachments := []EmailAttachment{
		{
			Filename:    "请求内容.txt",
			ContentType: "text/plain; charset=UTF-8",
			Data:        []byte("  原始内容\r\napi_key=sk-1234567890abcdefghijklmnop\n" + strings.Repeat("中文🙂", 2000) + "\n完整末尾  "),
		},
		{Filename: "empty.txt", ContentType: "text/plain; charset=UTF-8"},
		{Filename: "binary.dat", Data: []byte{0, 1, 2, 255}},
	}
	message, err := buildSMTPMessage(config, "user@example.net", "拦截通知", body, attachments...)
	require.NoError(t, err)
	parsed, err := mail.ReadMessage(bytes.NewReader(message.data))
	require.NoError(t, err)
	mediaType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/mixed", mediaType)
	require.NotEmpty(t, params["boundary"])
	require.Empty(t, parsed.Header.Get("Content-Transfer-Encoding"))
	reader := multipart.NewReader(parsed.Body, params["boundary"])
	part, err := reader.NextPart()
	require.NoError(t, err)
	require.Equal(t, "text/html; charset=UTF-8", part.Header.Get("Content-Type"))
	decodedBody, err := io.ReadAll(part)
	require.NoError(t, err)
	require.Equal(t, strings.ReplaceAll(body, "\n", "\r\n"), string(decodedBody))
	for _, want := range attachments {
		part, err := reader.NextPart()
		require.NoError(t, err)
		disposition, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		require.NoError(t, err)
		require.Equal(t, "attachment", disposition)
		require.Equal(t, want.Filename, params["filename"])
		contentType := want.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		require.Equal(t, contentType, part.Header.Get("Content-Type"))
		require.Equal(t, "base64", part.Header.Get("Content-Transfer-Encoding"))
		encoded, err := io.ReadAll(part)
		require.NoError(t, err)
		for _, line := range strings.Split(string(encoded), "\r\n") {
			require.LessOrEqual(t, len(line), 76, "attachment encoding must obey MIME line length limits")
		}
		decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(encoded)))
		require.NoError(t, err)
		require.True(t, bytes.Equal(want.Data, decoded), "attachment bytes must survive MIME encoding exactly")
	}
	_, err = reader.NextPart()
	require.ErrorIs(t, err, io.EOF)
}

func TestBuildSMTPMessageAttachmentHeaders(t *testing.T) {
	config := &SMTPConfig{Host: "smtp.example.com", From: "reply@example.com"}
	_, err := buildSMTPMessage(config, "user@example.net", "subject", "body", EmailAttachment{Filename: "\r\n"})
	require.ErrorContains(t, err, "filename is empty")
	_, err = buildSMTPMessage(config, "user@example.net", "subject", "body", EmailAttachment{
		Filename: "request.txt", ContentType: "text/plain\r\nBcc: hidden@example.com",
	})
	require.ErrorContains(t, err, "invalid email attachment content type")

	message, err := buildSMTPMessage(config, "user@example.net", "subject", "body", EmailAttachment{
		Filename: "request\r\nBcc: hidden@example.com.txt", Data: []byte("content"),
	})
	require.NoError(t, err)
	parsed, err := mail.ReadMessage(bytes.NewReader(message.data))
	require.NoError(t, err)
	require.Empty(t, parsed.Header.Get("Bcc"))
	_, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	require.NoError(t, err)
	reader := multipart.NewReader(parsed.Body, params["boundary"])
	_, err = reader.NextPart()
	require.NoError(t, err)
	part, err := reader.NextPart()
	require.NoError(t, err)
	require.Empty(t, part.Header.Get("Bcc"))
	_, params, err = mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	require.NoError(t, err)
	require.Equal(t, "requestBcc: hidden@example.com.txt", params["filename"])
}

func TestBuildSMTPMessagePreventsHeaderInjection(t *testing.T) {
	config := &SMTPConfig{
		Host:     "smtp.example.com",
		From:     "reply@example.com",
		FromName: "Sender\r\nBcc: hidden@example.com",
	}

	message, err := buildSMTPMessage(config, "user@example.net", "Subject\r\nCc: hidden@example.com", "body")
	require.NoError(t, err)

	parsed, err := mail.ReadMessage(bytes.NewReader(message.data))
	require.NoError(t, err)
	require.Empty(t, parsed.Header.Get("Bcc"))
	require.Empty(t, parsed.Header.Get("Cc"))

	decodedSubject, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	require.NoError(t, err)
	require.Equal(t, "SubjectCc: hidden@example.com", decodedSubject)
}

func TestBuildSMTPMessageRejectsInvalidConfiguration(t *testing.T) {
	_, err := buildSMTPMessage(nil, "user@example.net", "subject", "body")
	require.ErrorContains(t, err, "missing SMTP configuration")

	_, err = buildSMTPMessage(&SMTPConfig{Host: "smtp.example.com"}, "user@example.net", "subject", "body")
	require.ErrorContains(t, err, "invalid SMTP from address")

	_, err = buildSMTPMessage(&SMTPConfig{
		Host: "smtp.example.com",
		From: "reply@example.com",
	}, "invalid recipient <>", "subject", "body")
	require.ErrorContains(t, err, "invalid SMTP recipient address")

	_, err = buildSMTPMessage(&SMTPConfig{
		Host: "smtp.example.com",
		From: "reply@example.com",
	}, "user@example.net\r\nBcc: hidden@example.net", "subject", "body")
	require.ErrorContains(t, err, "invalid SMTP recipient address")
}

func TestBuildSMTPMessageUsesUniqueMessageIDs(t *testing.T) {
	config := &SMTPConfig{Host: "smtp.example.com", From: "reply@example.com"}

	first, err := buildSMTPMessage(config, "user@example.net", "subject", "body")
	require.NoError(t, err)
	second, err := buildSMTPMessage(config, "user@example.net", "subject", "body")
	require.NoError(t, err)

	firstParsed, err := mail.ReadMessage(bytes.NewReader(first.data))
	require.NoError(t, err)
	secondParsed, err := mail.ReadMessage(bytes.NewReader(second.data))
	require.NoError(t, err)
	require.NotEqual(t, firstParsed.Header.Get("Message-ID"), secondParsed.Header.Get("Message-ID"))
}
