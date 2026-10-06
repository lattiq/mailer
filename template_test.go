package mailer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateCID(t *testing.T) {
	engine, err := NewTemplateEngine(TemplateConfig{Enabled: true})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if err := engine.RegisterTemplate("logo.html", `<img src="{{cid .ID}}">`); err != nil {
		t.Fatalf("register: %v", err)
	}

	tests := map[string]string{
		"logo":            `<img src="cid:logo">`,
		"logo.png":        `<img src="cid:logo.png">`,
		"part1@lattiq":    `<img src="cid:part1@lattiq">`,
		`x" onerror="bad`: `<img src="cid:x%22%20onerror=%22bad">`,
	}
	for id, want := range tests {
		got, err := engine.Render("logo.html", map[string]string{"ID": id})
		if err != nil {
			t.Fatalf("render %q: %v", id, err)
		}
		if got != want {
			t.Errorf("cid %q: got %s, want %s", id, got, want)
		}
		if strings.Contains(got, "ZgotmplZ") {
			t.Errorf("cid %q was rejected by html/template: %s", id, got)
		}
	}
}

func TestLoadTemplatesFromDir(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "otp.html.html"), `<p>{{.OTP}}</p>`)
	mustWrite(t, filepath.Join(dir, "auth", "reset.text.text"), `code {{.OTP}}`)
	mustWrite(t, filepath.Join(dir, "notes.md"), `ignored: wrong extension`)

	engine, err := NewTemplateEngine(TemplateConfig{Enabled: true, Directory: dir, Extension: []string{".html", ".text"}})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	for name, want := range map[string]string{
		"otp.html":        "<p>42</p>",
		"auth.reset.text": "code 42",
	} {
		got, err := engine.Render(name, map[string]string{"OTP": "42"})
		if err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		if got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

// A symlink inside the template directory must not expose files outside it.
func TestLoadTemplatesFromDirRejectsSymlinkEscape(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.html")
	mustWrite(t, outside, `secret`)
	dir := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "leak.html")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := NewTemplateEngine(TemplateConfig{Enabled: true, Directory: dir, Extension: []string{".html"}}); err == nil {
		t.Fatal("expected an error loading a template symlinked outside the directory")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
