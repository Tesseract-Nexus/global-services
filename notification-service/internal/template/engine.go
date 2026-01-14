package template

import (
	"bytes"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
	"time"
)

// Engine handles template rendering
type Engine struct{}

// NewEngine creates a new template engine
func NewEngine() *Engine {
	return &Engine{}
}

// RenderText renders a text template with variables
func (e *Engine) RenderText(templateStr string, variables map[string]interface{}) (string, error) {
	tmpl, err := texttemplate.New("text").Funcs(e.funcMap()).Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RenderHTML renders an HTML template with variables
func (e *Engine) RenderHTML(templateStr string, variables map[string]interface{}) (string, error) {
	tmpl, err := htmltemplate.New("html").Funcs(e.htmlFuncMap()).Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (e *Engine) funcMap() texttemplate.FuncMap {
	return texttemplate.FuncMap{
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"title":      strings.Title,
		"trim":       strings.TrimSpace,
		"currency":   formatCurrency,
		"formatDate": formatDate,
		"default":    defaultValue,
	}
}

func (e *Engine) htmlFuncMap() htmltemplate.FuncMap {
	return htmltemplate.FuncMap{
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"title":      strings.Title,
		"trim":       strings.TrimSpace,
		"currency":   formatCurrency,
		"formatDate": formatDate,
		"default":    defaultValue,
		"safeHTML":   safeHTML,
	}
}

func formatCurrency(amount interface{}) string {
	switch v := amount.(type) {
	case float64:
		return formatFloat(v)
	case float32:
		return formatFloat(float64(v))
	case int:
		return formatFloat(float64(v))
	case int64:
		return formatFloat(float64(v))
	default:
		return "0.00"
	}
}

func formatFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(
		strings.Replace(
			strings.TrimSpace(
				string(append([]byte{}, []byte("$")...)),
			)+strings.Replace(
				strings.TrimRight(strings.TrimRight(
					func() string {
						s := make([]byte, 0, 32)
						s = append(s, []byte("$")...)
						// Simple formatting
						return string(s)
					}(), "0"), "."),
				"$", "", 1),
			"$", "", 1),
		"0"), ".")
}

func formatDate(t interface{}) string {
	switch v := t.(type) {
	case time.Time:
		return v.Format("January 2, 2006")
	case *time.Time:
		if v != nil {
			return v.Format("January 2, 2006")
		}
	case string:
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			return parsed.Format("January 2, 2006")
		}
	}
	return ""
}

func defaultValue(def, val interface{}) interface{} {
	if val == nil || val == "" {
		return def
	}
	return val
}

func safeHTML(s string) htmltemplate.HTML {
	return htmltemplate.HTML(s)
}
