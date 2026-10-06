// Package message renders a core.Email as an RFC 5322 message with a MIME
// body, for providers that send raw messages (SMTP, SES SendRawEmail).
package message

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"sort"
	"strings"
	"time"

	"github.com/lattiq/mailer/internal/core"
)

// Build renders email as an RFC 5322 message. Bcc recipients are not written
// to the headers; the caller passes them to the transport as envelope
// recipients.
func Build(email *core.Email) ([]byte, error) {
	var message strings.Builder

	// Headers
	message.WriteString("From: " + email.From.String() + "\r\n")

	if len(email.To) > 0 {
		var toAddrs []string
		for _, to := range email.To {
			toAddrs = append(toAddrs, to.String())
		}
		message.WriteString("To: " + strings.Join(toAddrs, ", ") + "\r\n")
	}

	if len(email.CC) > 0 {
		var ccAddrs []string
		for _, cc := range email.CC {
			ccAddrs = append(ccAddrs, cc.String())
		}
		message.WriteString("Cc: " + strings.Join(ccAddrs, ", ") + "\r\n")
	}

	// Checked again here, not only in Email.Validate: a line break in a header
	// value would let it inject arbitrary headers into the raw message.
	if strings.ContainsAny(email.Subject, "\r\n") {
		return nil, fmt.Errorf("subject must not contain line breaks")
	}

	// Non-ASCII text must be RFC 2047 encoded; ASCII passes through unchanged.
	message.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", email.Subject) + "\r\n")
	message.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	message.WriteString("MIME-Version: 1.0\r\n")

	// Custom and priority headers, sorted so the message is reproducible.
	headers := email.HeadersWithPriority()
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := headers[name]
		if err := core.ValidateHeader(name, value); err != nil {
			return nil, err
		}
		message.WriteString(name + ": " + mime.QEncoding.Encode("UTF-8", value) + "\r\n")
	}

	body, err := buildBody(email)
	if err != nil {
		return nil, err
	}
	writeHeader(&message, body.header)
	message.WriteString("\r\n")
	message.Write(body.content)

	return []byte(message.String()), nil
}

// mimePart is one encoded MIME entity: its headers and its encoded content.
type mimePart struct {
	header  textproto.MIMEHeader
	content []byte
}

// buildBody assembles the MIME tree, adding each layer only when needed:
//
//	multipart/mixed           regular attachments
//	└ multipart/related       inline images referenced as cid: from the HTML
//	  └ multipart/alternative text and HTML bodies
func buildBody(email *core.Email) (mimePart, error) {
	var body mimePart
	switch {
	case email.HTMLBody != "" && email.TextBody != "":
		body = multipartOf("alternative",
			textPart("text/plain", email.TextBody),
			textPart("text/html", email.HTMLBody))
	case email.HTMLBody != "":
		body = textPart("text/html", email.HTMLBody)
	default:
		body = textPart("text/plain", email.TextBody)
	}

	var inline, regular []mimePart
	for i := range email.Attachments {
		part, err := attachmentPart(&email.Attachments[i])
		if err != nil {
			return mimePart{}, err
		}
		if email.Attachments[i].Inline {
			inline = append(inline, part)
		} else {
			regular = append(regular, part)
		}
	}

	if len(inline) > 0 {
		body = multipartOf("related", append([]mimePart{body}, inline...)...)
	}
	if len(regular) > 0 {
		body = multipartOf("mixed", append([]mimePart{body}, regular...)...)
	}
	return body, nil
}

// textPart encodes body as quoted-printable to match its
// Content-Transfer-Encoding header. Writing it raw lets clients decode every
// "=" as an escape, which mangles HTML attributes (class="x", src="...").
func textPart(contentType, body string) mimePart {
	var buf bytes.Buffer
	w := quotedprintable.NewWriter(&buf)
	// Writes to a bytes.Buffer cannot fail.
	_, _ = w.Write([]byte(body))
	_ = w.Close()

	header := textproto.MIMEHeader{}
	header.Set("Content-Type", contentType+"; charset=UTF-8")
	header.Set("Content-Transfer-Encoding", "quoted-printable")
	return mimePart{header: header, content: buf.Bytes()}
}

// attachmentPart base64-encodes an attachment.
func attachmentPart(att *core.Attachment) (mimePart, error) {
	data, err := att.ReadData()
	if err != nil {
		return mimePart{}, err
	}

	disposition := "attachment"
	if att.Inline {
		disposition = "inline"
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Type", mime.FormatMediaType(att.DetectContentType(), map[string]string{"name": att.Filename}))
	header.Set("Content-Transfer-Encoding", "base64")
	header.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": att.Filename}))
	if att.Inline {
		header.Set("Content-ID", "<"+att.InlineContentID()+">")
	}

	// RFC 2045 limits base64 lines to 76 characters.
	encoded := base64.StdEncoding.EncodeToString(data)
	var buf bytes.Buffer
	for len(encoded) > 76 {
		buf.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	buf.WriteString(encoded + "\r\n")
	return mimePart{header: header, content: buf.Bytes()}, nil
}

// multipartOf wraps parts in a multipart/<subtype> entity with a random
// boundary.
func multipartOf(subtype string, parts ...mimePart) mimePart {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, part := range parts {
		// Writes to a bytes.Buffer cannot fail.
		w, _ := mw.CreatePart(part.header)
		_, _ = w.Write(part.content)
	}
	_ = mw.Close()

	header := textproto.MIMEHeader{}
	header.Set("Content-Type", mime.FormatMediaType("multipart/"+subtype, map[string]string{"boundary": mw.Boundary()}))
	return mimePart{header: header, content: buf.Bytes()}
}

// writeHeader writes MIME headers in a stable order.
func writeHeader(message *strings.Builder, header textproto.MIMEHeader) {
	keys := make([]string, 0, len(header))
	for k := range header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range header[k] {
			message.WriteString(k + ": " + v + "\r\n")
		}
	}
}
