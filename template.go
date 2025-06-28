package mailer

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	textTemplate "text/template"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// TemplateEngineImpl implements the TemplateEngine interface.
type TemplateEngineImpl struct {
	config        TemplateConfig
	htmlTemplates map[string]*template.Template
	textTemplates map[string]*textTemplate.Template
	mutex         sync.RWMutex
}

// NewTemplateEngine creates a new template engine with the given configuration.
func NewTemplateEngine(config TemplateConfig) (TemplateEngine, error) {
	engine := &TemplateEngineImpl{
		config:        config,
		htmlTemplates: make(map[string]*template.Template),
		textTemplates: make(map[string]*textTemplate.Template),
	}

	// Load templates from directory if specified
	if config.Directory != "" {
		if err := engine.LoadTemplatesFromDir(config.Directory); err != nil {
			return nil, fmt.Errorf("failed to load templates from directory: %w", err)
		}
	}

	return engine, nil
}

// Render renders a template with the provided data.
func (te *TemplateEngineImpl) Render(templateName string, data interface{}) (string, error) {
	te.mutex.RLock()
	defer te.mutex.RUnlock()

	// Try HTML template first
	if htmlTmpl, exists := te.htmlTemplates[templateName]; exists {
		var buf strings.Builder
		if err := htmlTmpl.Execute(&buf, data); err != nil {
			return "", NewTemplateError(templateName, "render", "failed to execute HTML template", err)
		}
		return buf.String(), nil
	}

	// Try text template
	if textTmpl, exists := te.textTemplates[templateName]; exists {
		var buf strings.Builder
		if err := textTmpl.Execute(&buf, data); err != nil {
			return "", NewTemplateError(templateName, "render", "failed to execute text template", err)
		}
		return buf.String(), nil
	}

	return "", ErrTemplateNotFound
}

// RegisterTemplate registers a template with the given name and content.
func (te *TemplateEngineImpl) RegisterTemplate(name string, content string) error {
	te.mutex.Lock()
	defer te.mutex.Unlock()

	// Determine template type from name or content
	if strings.Contains(name, ".html") || strings.Contains(content, "<") {
		// HTML template
		tmpl, err := template.New(name).Funcs(te.getTemplateFuncs()).Parse(content)
		if err != nil {
			return NewTemplateError(name, "parse", "failed to parse HTML template", err)
		}
		te.htmlTemplates[name] = tmpl
	} else {
		// Text template
		tmpl, err := textTemplate.New(name).Funcs(te.getTextTemplateFuncs()).Parse(content)
		if err != nil {
			return NewTemplateError(name, "parse", "failed to parse text template", err)
		}
		te.textTemplates[name] = tmpl
	}

	return nil
}

// LoadTemplatesFromDir loads all templates from the specified directory.
func (te *TemplateEngineImpl) LoadTemplatesFromDir(dir string) error {
	// Clean and validate the directory path
	cleanDir := filepath.Clean(dir)

	return filepath.WalkDir(cleanDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Security: Validate that the path is within the specified directory
		cleanPath := filepath.Clean(path)
		if !isPathWithinDir(cleanPath, cleanDir) {
			return fmt.Errorf("security error: path traversal detected: %s", path)
		}

		// Check if file has a valid template extension
		ext := filepath.Ext(path)
		validExt := false
		for _, validExtension := range te.config.Extension {
			if ext == validExtension {
				validExt = true
				break
			}
		}

		if !validExt {
			return nil
		}

		// Read template file with validated path
		content, err := os.ReadFile(cleanPath)
		if err != nil {
			return fmt.Errorf("failed to read template file %s: %w", cleanPath, err)
		}

		// Generate template name from file path
		relativePath, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Remove extension from template name
		templateName := strings.TrimSuffix(relativePath, ext)

		// Replace path separators with dots for hierarchical templates
		templateName = strings.ReplaceAll(templateName, string(filepath.Separator), ".")

		// Register the template
		if err := te.RegisterTemplate(templateName, string(content)); err != nil {
			return fmt.Errorf("failed to register template %s: %w", templateName, err)
		}

		return nil
	})
}

// getTemplateFuncs returns the template functions for HTML templates.
func (te *TemplateEngineImpl) getTemplateFuncs() template.FuncMap {
	titleCaser := cases.Title(language.English)
	funcs := template.FuncMap{
		"upper":     strings.ToUpper,
		"lower":     strings.ToLower,
		"title":     titleCaser.String,
		"trim":      strings.TrimSpace,
		"join":      strings.Join,
		"split":     strings.Split,
		"replace":   strings.ReplaceAll,
		"contains":  strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"now":       time.Now,
		"formatTime": func(format string, t time.Time) string {
			return t.Format(format)
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"mul": func(a, b int) int {
			return a * b
		},
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mod": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a % b
		},
		"eq":  func(a, b interface{}) bool { return a == b },
		"ne":  func(a, b interface{}) bool { return a != b },
		"lt":  func(a, b int) bool { return a < b },
		"le":  func(a, b int) bool { return a <= b },
		"gt":  func(a, b int) bool { return a > b },
		"ge":  func(a, b int) bool { return a >= b },
		"and": func(a, b bool) bool { return a && b },
		"or":  func(a, b bool) bool { return a || b },
		"not": func(a bool) bool { return !a },
		"default": func(defaultValue, value interface{}) interface{} {
			if value == nil || value == "" {
				return defaultValue
			}
			return value
		},
	}

	// Only add unsafe functions if explicitly enabled in config
	if te.config.AllowUnsafeFunctions {
		// SECURITY WARNING: These functions bypass Go's auto-escaping and can lead to XSS
		// Only use these functions with trusted, validated content
		funcs["unsafeHTML"] = func(s string) template.HTML {
			// Only use this with content you trust completely
			return template.HTML(s) // #nosec G203 -- Intentionally unsafe, opt-in only
		}
		funcs["unsafeCSS"] = func(s string) template.CSS {
			// Only use this with content you trust completely
			return template.CSS(s) // #nosec G203 -- Intentionally unsafe, opt-in only
		}
		funcs["unsafeJS"] = func(s string) template.JS {
			// Only use this with content you trust completely
			return template.JS(s) // #nosec G203 -- Intentionally unsafe, opt-in only
		}
		funcs["unsafeURL"] = func(s string) template.URL {
			// Only use this with content you trust completely
			return template.URL(s) // #nosec G203 -- Intentionally unsafe, opt-in only
		}
	}

	return funcs
}

// getTextTemplateFuncs returns the template functions for text templates.
func (te *TemplateEngineImpl) getTextTemplateFuncs() textTemplate.FuncMap {
	titleCaser := cases.Title(language.English)
	return textTemplate.FuncMap{
		"upper":     strings.ToUpper,
		"lower":     strings.ToLower,
		"title":     titleCaser.String,
		"trim":      strings.TrimSpace,
		"join":      strings.Join,
		"split":     strings.Split,
		"replace":   strings.ReplaceAll,
		"contains":  strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"now":       time.Now,
		"formatTime": func(format string, t time.Time) string {
			return t.Format(format)
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"mul": func(a, b int) int {
			return a * b
		},
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mod": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a % b
		},
		"eq":  func(a, b interface{}) bool { return a == b },
		"ne":  func(a, b interface{}) bool { return a != b },
		"lt":  func(a, b int) bool { return a < b },
		"le":  func(a, b int) bool { return a <= b },
		"gt":  func(a, b int) bool { return a > b },
		"ge":  func(a, b int) bool { return a >= b },
		"and": func(a, b bool) bool { return a && b },
		"or":  func(a, b bool) bool { return a || b },
		"not": func(a bool) bool { return !a },
		"default": func(defaultValue, value interface{}) interface{} {
			if value == nil || value == "" {
				return defaultValue
			}
			return value
		},
	}
}

// isPathWithinDir checks if a given path is within the specified directory to prevent path traversal attacks.
func isPathWithinDir(path, dir string) bool {
	// Get absolute paths
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}

	// Check if the path starts with the directory
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}

	// If rel starts with "..", it's outside the directory
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

// Close closes the template engine and releases any resources.
func (te *TemplateEngineImpl) Close() error {
	te.mutex.Lock()
	defer te.mutex.Unlock()

	// Clear template caches
	te.htmlTemplates = make(map[string]*template.Template)
	te.textTemplates = make(map[string]*textTemplate.Template)

	return nil
}
