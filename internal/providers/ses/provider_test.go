package ses

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"

	"github.com/lattiq/mailer/internal/core"
)

// fakeSES records the API action and form of each request and answers with
// a minimal successful response.
type fakeSES struct {
	actions []string
	form    url.Values
}

func (f *fakeSES) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	action := r.Form.Get("Action")
	f.actions = append(f.actions, action)
	f.form = r.Form
	w.Header().Set("Content-Type", "text/xml")
	_, _ = fmt.Fprintf(w, `<%[1]sResponse><%[1]sResult><MessageId>test-id</MessageId></%[1]sResult></%[1]sResponse>`, action)
}

func newTestProvider(t *testing.T, fake *fakeSES) *Provider {
	t.Helper()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client := ses.New(ses.Options{
		Region:       "ap-south-1",
		BaseEndpoint: aws.String(server.URL),
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
		}),
	})
	return &Provider{client: client, config: core.ProviderSettings{}}
}

func TestSendChoosesAPI(t *testing.T) {
	tests := []struct {
		name  string
		email core.Email
		want  string
	}{
		{"plain", core.Email{}, "SendEmail"},
		{"custom headers", core.Email{Headers: map[string]string{"X-Category": "otp"}}, "SendRawEmail"},
		{"high priority", core.Email{Priority: core.PriorityHigh}, "SendRawEmail"},
		{"attachment", core.Email{Attachments: []core.Attachment{
			{Filename: "logo.png", Data: bytes.NewReader([]byte("png")), Inline: true, ContentID: "logo"},
		}}, "SendRawEmail"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeSES{}
			p := newTestProvider(t, fake)
			email := tt.email
			email.From = core.Address{Email: "from@example.com"}
			email.To = []core.Address{{Email: "to@example.com"}}
			email.Subject = "test"
			email.HTMLBody = "<p>hi</p>"

			if _, err := p.Send(context.Background(), &email); err != nil {
				t.Fatalf("send: %v", err)
			}
			if len(fake.actions) != 1 || fake.actions[0] != tt.want {
				t.Fatalf("got actions %v, want [%s]", fake.actions, tt.want)
			}
		})
	}
}

// The raw message omits Bcc from its headers, so Bcc must be passed as an
// envelope destination or those recipients never get the email.
func TestSendRawIncludesAllDestinations(t *testing.T) {
	fake := &fakeSES{}
	p := newTestProvider(t, fake)
	email := &core.Email{
		From:     core.Address{Email: "from@example.com"},
		To:       []core.Address{{Email: "to@example.com"}},
		CC:       []core.Address{{Email: "cc@example.com"}},
		BCC:      []core.Address{{Email: "bcc@example.com"}},
		Subject:  "test",
		HTMLBody: "<p>hi</p>",
		Headers:  map[string]string{"X-Category": "otp"},
	}
	if _, err := p.Send(context.Background(), email); err != nil {
		t.Fatalf("send: %v", err)
	}

	var destinations []string
	for i := 1; ; i++ {
		d := fake.form.Get(fmt.Sprintf("Destinations.member.%d", i))
		if d == "" {
			break
		}
		destinations = append(destinations, d)
	}
	if got := strings.Join(destinations, ","); got != "to@example.com,cc@example.com,bcc@example.com" {
		t.Errorf("got destinations %q", got)
	}

	raw, err := base64.StdEncoding.DecodeString(fake.form.Get("RawMessage.Data"))
	if err != nil {
		t.Fatalf("decode raw message: %v", err)
	}
	if bytes.Contains(raw, []byte("bcc@example.com")) {
		t.Error("raw message exposes the Bcc recipient")
	}
	if !bytes.Contains(raw, []byte("X-Category: otp")) {
		t.Error("raw message is missing the custom header")
	}
}
