package mailgun

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lattiq/mailer/internal/core"
)

// Sends through a fake Mailgun API and checks the multipart form: inline
// files go in "inline" named after their Content-ID, the rest in "attachment".
func TestSendAttachments(t *testing.T) {
	logo := []byte{0x89, 'P', 'N', 'G', 0x00, 0xff}
	pdf := []byte("%PDF-1.4 fake")

	type file struct {
		name string
		data []byte
	}
	var requests int
	got := map[string][]file{}
	var importance []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse form: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		got = map[string][]file{}
		importance = r.MultipartForm.Value["h:Importance"]
		for field, headers := range r.MultipartForm.File {
			for _, h := range headers {
				f, err := h.Open()
				if err != nil {
					t.Errorf("open %s: %v", h.Filename, err)
					continue
				}
				data, _ := io.ReadAll(f)
				_ = f.Close()
				got[field] = append(got[field], file{h.Filename, data})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"<test@example.com>","message":"Queued"}`))
	}))
	defer server.Close()

	p, err := NewProvider(core.ProviderSettings{
		"api_key":  "key-test",
		"domain":   "example.com",
		"base_url": server.URL + "/v3",
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	email := &core.Email{
		From:     core.Address{Email: "from@example.com"},
		To:       []core.Address{{Email: "to@example.com"}},
		Subject:  "test",
		TextBody: "text",
		HTMLBody: `<img src="cid:logo.png" />`,
		Attachments: []core.Attachment{
			{Filename: "transunion.png", Data: bytes.NewReader(logo), Inline: true, ContentID: "logo.png"},
			{Filename: "report.pdf", Data: bytes.NewReader(pdf)},
		},
	}

	// Send twice: a retry must re-read the seekable attachment data.
	for i := range 2 {
		if _, err := p.Send(context.Background(), email); err != nil {
			t.Fatalf("send %d: %v", i+1, err)
		}
	}
	if requests != 2 {
		t.Fatalf("got %d requests, want 2", requests)
	}

	check := func(field, name string, want []byte) {
		t.Helper()
		files := got[field]
		if len(files) != 1 {
			t.Fatalf("%s: got %d files, want 1", field, len(files))
		}
		if files[0].name != name {
			t.Errorf("%s: got name %q, want %q", field, files[0].name, name)
		}
		if !bytes.Equal(files[0].data, want) {
			t.Errorf("%s: got %d bytes, want %d", field, len(files[0].data), len(want))
		}
	}
	// No priority set: Mailgun used to mark such mail as low importance.
	if len(importance) != 0 {
		t.Errorf("got Importance %v for an email with no priority", importance)
	}
	check("inline", "logo.png", logo)
	check("attachment", "report.pdf", pdf)
}

func TestSendRequiresRecipient(t *testing.T) {
	p, err := NewProvider(core.ProviderSettings{"api_key": "key-test", "domain": "example.com"})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if _, err := p.Send(context.Background(), &core.Email{From: core.Address{Email: "from@example.com"}}); err == nil {
		t.Fatal("expected an error for an email with no recipients")
	}
}
