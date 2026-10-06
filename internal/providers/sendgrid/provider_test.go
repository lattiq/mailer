package sendgrid

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/lattiq/mailer/internal/core"
)

func TestBuildMessageAttachments(t *testing.T) {
	logo := []byte{0x89, 'P', 'N', 'G', 0x00, 0xff}
	pdf := []byte("%PDF-1.4 fake")
	email := &core.Email{
		From:     core.Address{Email: "from@example.com"},
		To:       []core.Address{{Email: "to@example.com"}},
		Subject:  "test",
		HTMLBody: `<img src="cid:logo" />`,
		Attachments: []core.Attachment{
			{Filename: "logo.png", Data: bytes.NewReader(logo), Inline: true, ContentID: "logo"},
			{Filename: "report.pdf", Data: bytes.NewReader(pdf)},
		},
	}

	// Build twice: a retry must re-read the seekable attachment data.
	if _, err := buildMessage(email); err != nil {
		t.Fatalf("first build: %v", err)
	}
	message, err := buildMessage(email)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}

	// Assert on the JSON payload SendGrid receives.
	var payload struct {
		Attachments []struct {
			Content     string `json:"content"`
			Type        string `json:"type"`
			Filename    string `json:"filename"`
			Disposition string `json:"disposition"`
			ContentID   string `json:"content_id"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(mail.GetRequestBody(message), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload.Attachments) != 2 {
		t.Fatalf("got %d attachments, want 2", len(payload.Attachments))
	}

	tests := []struct {
		filename, contentType, disposition, contentID string
		data                                          []byte
	}{
		{"logo.png", "image/png", "inline", "logo", logo},
		{"report.pdf", "application/pdf", "attachment", "", pdf},
	}
	for i, want := range tests {
		got := payload.Attachments[i]
		if got.Filename != want.filename || got.Type != want.contentType ||
			got.Disposition != want.disposition || got.ContentID != want.contentID {
			t.Errorf("attachment %d: got %+v, want %+v", i, got, want)
		}
		data, err := base64.StdEncoding.DecodeString(got.Content)
		if err != nil {
			t.Fatalf("attachment %d: decode content: %v", i, err)
		}
		if !bytes.Equal(data, want.data) {
			t.Errorf("attachment %d: got %d bytes, want %d", i, len(data), len(want.data))
		}
	}
}
