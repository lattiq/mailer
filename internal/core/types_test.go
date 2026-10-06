package core

import (
	"errors"
	"testing"
)

func validEmail() *Email {
	return &Email{
		From:     Address{Email: "from@example.com"},
		To:       []Address{{Email: "to@example.com"}},
		Subject:  "subject",
		TextBody: "body",
	}
}

func TestValidateHeaders(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		headers map[string]string
		wantErr bool
	}{
		{name: "plain", headers: map[string]string{"X-Category": "otp"}},
		{name: "non-ASCII value", headers: map[string]string{"X-Note": "₹ தமிழ்"}},
		{name: "subject CRLF injection", subject: "Hi\r\nBcc: evil@example.com", wantErr: true},
		{name: "subject bare LF", subject: "Hi\nthere", wantErr: true},
		{name: "header value injection", headers: map[string]string{"X-Note": "a\r\nBcc: evil@example.com"}, wantErr: true},
		{name: "header name with colon", headers: map[string]string{"X:Bad": "v"}, wantErr: true},
		{name: "header name with space", headers: map[string]string{"X Bad": "v"}, wantErr: true},
		{name: "empty header name", headers: map[string]string{"": "v"}, wantErr: true},
		{name: "reserved header", headers: map[string]string{"bcc": "evil@example.com"}, wantErr: true},
		{name: "reserved MIME header", headers: map[string]string{"Content-Type": "text/plain"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validEmail()
			if tt.subject != "" {
				e.Subject = tt.subject
			}
			e.Headers = tt.headers

			err := e.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			var vErr *ValidationError
			if err != nil && !errors.As(err, &vErr) {
				t.Errorf("got %T, want *ValidationError", err)
			}
		})
	}
}

func TestPriorityHeaders(t *testing.T) {
	tests := []struct {
		priority Priority
		custom   map[string]string
		want     map[string]string
	}{
		// PriorityLow is the zero value, so unset priority must add nothing.
		{priority: PriorityLow, want: nil},
		{priority: PriorityNormal, want: nil},
		{priority: PriorityHigh, want: map[string]string{"X-Priority": "2", "Importance": "high"}},
		{priority: PriorityUrgent, want: map[string]string{"X-Priority": "1", "Importance": "high"}},
		// Custom headers win, matched case-insensitively.
		{
			priority: PriorityUrgent,
			custom:   map[string]string{"x-priority": "3", "X-Category": "otp"},
			want:     map[string]string{"x-priority": "3", "Importance": "high", "X-Category": "otp"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.priority.String(), func(t *testing.T) {
			e := &Email{Priority: tt.priority, Headers: tt.custom}
			got := e.HeadersWithPriority()
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for name, value := range tt.want {
				if got[name] != value {
					t.Errorf("%s: got %q, want %q", name, got[name], value)
				}
			}
		})
	}
}
