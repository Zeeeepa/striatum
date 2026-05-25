package webassets

import (
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"mime"
	"path"
	"strings"
)

const ContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'"

//go:embed static/* templates/*
var embedded embed.FS

type Asset struct {
	Body        []byte
	ContentType string
}

func LoadStatic(relative string) (Asset, error) {
	clean, err := cleanRelative(relative)
	if err != nil {
		return Asset{}, err
	}
	body, err := embedded.ReadFile(path.Join("static", clean))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Asset{}, fs.ErrNotExist
		}
		return Asset{}, err
	}
	return Asset{Body: body, ContentType: contentType(clean)}, nil
}

func RenderPage(title string, payload any) ([]byte, error) {
	tmpl, err := template.ParseFS(embedded, "templates/page.html")
	if err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	var builder strings.Builder
	err = tmpl.Execute(&builder, map[string]any{
		"Title":   title,
		"Payload": string(encoded),
	})
	if err != nil {
		return nil, err
	}
	return []byte(builder.String()), nil
}

func cleanRelative(relative string) (string, error) {
	if relative == "" || strings.HasPrefix(relative, "/") {
		return "", fs.ErrInvalid
	}
	clean := path.Clean(relative)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fs.ErrInvalid
	}
	return clean, nil
}

func contentType(relative string) string {
	if value := mime.TypeByExtension(path.Ext(relative)); value != "" {
		return value
	}
	switch strings.ToLower(path.Ext(relative)) {
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
