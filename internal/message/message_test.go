package message

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"

	"github.com/lattiq/mailer/internal/core"
)

// Bodies must survive a real MIME decode unchanged: they are declared
// quoted-printable, so any "=" written raw gets misread as an escape.
func TestBuildMessageQuotedPrintableRoundTrip(t *testing.T) {
	html := `<div class="otp-code">123456</div><img src="https://example.com/logo.png?a=1&b=2" />` +
		strings.Repeat("x", 200) + " © 2026"
	text := "Your code is 123456 (a=b) © 2026\n" + strings.Repeat("y", 200)

	raw, err := Build(&core.Email{
		From:     core.Address{Email: "from@example.com"},
		To:       []core.Address{{Email: "to@example.com"}},
		Subject:  "test",
		TextBody: text,
		HTMLBody: html,
	})
	if err != nil {
		t.Fatalf("build message: %v", err)
	}

	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 998 {
			t.Fatalf("line exceeds RFC 5322 limit: %d chars", len(line))
		}
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}
	_, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}

	got := map[string]string{}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	for {
		part, err := mr.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		if enc := part.Header.Get("Content-Transfer-Encoding"); enc != "quoted-printable" {
			t.Fatalf("unexpected encoding %q", enc)
		}
		body, err := io.ReadAll(quotedprintable.NewReader(part))
		if err != nil {
			t.Fatalf("decode part: %v", err)
		}
		ct, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		// The part ends with the CRLF that precedes the next boundary.
		got[ct] = strings.TrimSuffix(string(body), "\r\n")
	}

	if got["text/html"] != html {
		t.Errorf("html body mismatch:\n got: %q\nwant: %q", got["text/html"], html)
	}
	// The QP writer normalises line endings to CRLF in text mode.
	if want := strings.ReplaceAll(text, "\n", "\r\n"); got["text/plain"] != want {
		t.Errorf("text body mismatch:\n got: %q\nwant: %q", got["text/plain"], want)
	}
}

// Inline images must sit in multipart/related next to the HTML so clients
// resolve cid: references; regular attachments go in an outer multipart/mixed.
func TestBuildMessageAttachments(t *testing.T) {
	logo := bytes.Repeat([]byte{0x89, 'P', 'N', 'G', 0x00, 0xff}, 100)
	pdf := []byte("%PDF-1.4 fake")
	email := &core.Email{
		From:     core.Address{Email: "from@example.com"},
		To:       []core.Address{{Email: "to@example.com"}},
		Subject:  "test",
		TextBody: "text",
		HTMLBody: `<img src="cid:logo" />`,
		Attachments: []core.Attachment{
			{Filename: "logo.png", Data: bytes.NewReader(logo), Inline: true, ContentID: "logo"},
			{Filename: "report.pdf", Data: bytes.NewReader(pdf)},
		},
	}

	// Build twice: a retry must re-read the seekable attachment data.
	if _, err := Build(email); err != nil {
		t.Fatalf("first build: %v", err)
	}
	raw, err := Build(email)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}

	mixed := readParts(t, msg.Header.Get("Content-Type"), msg.Body, "multipart/mixed")
	if len(mixed) != 2 {
		t.Fatalf("mixed: got %d parts, want 2", len(mixed))
	}
	assertAttachment(t, mixed[1], "application/pdf", "attachment", "", pdf)

	related := readParts(t, mixed[0].header.Get("Content-Type"), bytes.NewReader(mixed[0].body), "multipart/related")
	if len(related) != 2 {
		t.Fatalf("related: got %d parts, want 2", len(related))
	}
	assertAttachment(t, related[1], "image/png", "inline", "<logo>", logo)

	alternative := readParts(t, related[0].header.Get("Content-Type"), bytes.NewReader(related[0].body), "multipart/alternative")
	if len(alternative) != 2 {
		t.Fatalf("alternative: got %d parts, want 2", len(alternative))
	}
}

type rawPart struct {
	header textproto.MIMEHeader
	body   []byte
}

func readParts(t *testing.T, contentType string, r io.Reader, want string) []rawPart {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type %q: %v", contentType, err)
	}
	if mediaType != want {
		t.Fatalf("got %s, want %s", mediaType, want)
	}
	var parts []rawPart
	mr := multipart.NewReader(r, params["boundary"])
	for {
		part, err := mr.NextRawPart()
		if err == io.EOF {
			return parts
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		parts = append(parts, rawPart{header: part.Header, body: body})
	}
}

func assertAttachment(t *testing.T, part rawPart, contentType, disposition, contentID string, want []byte) {
	t.Helper()
	if ct, _, _ := mime.ParseMediaType(part.header.Get("Content-Type")); ct != contentType {
		t.Errorf("content type: got %q, want %q", ct, contentType)
	}
	if d, _, _ := mime.ParseMediaType(part.header.Get("Content-Disposition")); d != disposition {
		t.Errorf("disposition: got %q, want %q", d, disposition)
	}
	if got := part.header.Get("Content-ID"); got != contentID {
		t.Errorf("content id: got %q, want %q", got, contentID)
	}
	for _, line := range strings.Split(string(part.body), "\r\n") {
		if len(line) > 76 {
			t.Fatalf("base64 line exceeds 76 chars: %d", len(line))
		}
	}
	got, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(part.body), "\r\n", ""))
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("attachment data mismatch: got %d bytes, want %d", len(got), len(want))
	}
}

func TestBuildEncodesHeaders(t *testing.T) {
	raw, err := Build(&core.Email{
		From:     core.Address{Email: "from@example.com"},
		To:       []core.Address{{Email: "to@example.com"}},
		Subject:  "Your code ₹ தமிழ்",
		TextBody: "text",
		Headers:  map[string]string{"X-Zeta": "z", "X-Alpha": "நன்றி", "X-Mid": "plain"},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	head, _, _ := strings.Cut(string(raw), "\r\n\r\n")
	for _, line := range strings.Split(head, "\r\n") {
		for _, b := range []byte(line) {
			if b > 127 {
				t.Fatalf("header line has raw non-ASCII bytes: %q", line)
			}
		}
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}
	dec := new(mime.WordDecoder)
	for name, want := range map[string]string{"Subject": "Your code ₹ தமிழ்", "X-Alpha": "நன்றி", "X-Mid": "plain"} {
		got, err := dec.DecodeHeader(msg.Header.Get(name))
		if err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		if got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}

	// Custom headers are written in sorted order.
	alpha, mid, zeta := strings.Index(head, "X-Alpha:"), strings.Index(head, "X-Mid:"), strings.Index(head, "X-Zeta:")
	if alpha < 0 || mid < 0 || zeta < 0 || alpha > mid || mid > zeta {
		t.Errorf("custom headers not sorted: X-Alpha@%d X-Mid@%d X-Zeta@%d", alpha, mid, zeta)
	}
}

// Build is the last step before bytes go on the wire, so it rejects header
// injection even if Email.Validate was skipped.
func TestBuildRejectsLineBreaks(t *testing.T) {
	tests := map[string]*core.Email{
		"subject": {Subject: "Hi\r\nBcc: evil@example.com", TextBody: "x"},
		"header":  {Subject: "Hi", TextBody: "x", Headers: map[string]string{"X-Note": "a\nBcc: evil@example.com"}},
	}
	for name, email := range tests {
		t.Run(name, func(t *testing.T) {
			email.From = core.Address{Email: "from@example.com"}
			email.To = []core.Address{{Email: "to@example.com"}}
			if _, err := Build(email); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestBuildPriorityHeaders(t *testing.T) {
	for _, tt := range []struct {
		priority core.Priority
		want     string
	}{
		{core.PriorityLow, ""},
		{core.PriorityUrgent, "1"},
	} {
		raw, err := Build(&core.Email{
			From:     core.Address{Email: "from@example.com"},
			To:       []core.Address{{Email: "to@example.com"}},
			Subject:  "test",
			TextBody: "text",
			Priority: tt.priority,
		})
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		msg, err := mail.ReadMessage(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := msg.Header.Get("X-Priority"); got != tt.want {
			t.Errorf("%s: X-Priority = %q, want %q", tt.priority, got, tt.want)
		}
	}
}
