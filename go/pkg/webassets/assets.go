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
		"Data":    payload,
	})
	if err != nil {
		return nil, err
	}
	return []byte(builder.String()), nil
}

// InterrogationMeta carries the curated header fields for an interrogation
// thread. Only fields the existing read projects are present (D028): no
// provider stdout/stderr, no raw captured output, no tool payloads.
type InterrogationMeta struct {
	InterrogationID       string
	RunID                 string
	Topic                 string
	State                 string
	InterrogatorSessionID string
	TargetSessionID       string
	OpenedAt              string
	ClosedAt              string
}

// InterrogationTurn is one curated Q&A turn: the message kind enum and the
// authored body text. Nothing else from the message bus is rendered.
type InterrogationTurn struct {
	Kind string
	Body string
}

// RenderInterrogation renders an interrogation thread as a chat thread.
//
// It uses html/template (NOT text/template): each turn Body is bound as
// {{.Body}} in HTML element context, so html/template context-aware
// auto-escaping neutralizes stored XSS / scriptable HTML injection before the
// bytes leave the process. Bodies are rendered as plain text — Markdown is
// deliberately not parsed — so links, code blocks, and raw HTML all display as
// inert, escaped characters. This is the single render-boundary enforcement
// point for both the escaping guarantee and the curated-field guarantee.
func RenderInterrogation(meta InterrogationMeta, turns []InterrogationTurn) ([]byte, error) {
	tmpl, err := template.ParseFS(embedded, "templates/interrogation.html")
	if err != nil {
		return nil, err
	}
	type viewTurn struct {
		Speaker  string
		Body     string
		Question bool
	}
	view := struct {
		InterrogationMeta
		Turns []viewTurn
	}{InterrogationMeta: meta}
	for _, turn := range turns {
		vt := viewTurn{Body: turn.Body}
		if turn.Kind == "interrogation_answer" {
			vt.Speaker = "Target"
			vt.Question = false
		} else {
			// interrogation_question (and any unexpected kind) is the reviewer.
			vt.Speaker = "Reviewer"
			vt.Question = true
		}
		view.Turns = append(view.Turns, vt)
	}
	var builder strings.Builder
	if err := tmpl.Execute(&builder, view); err != nil {
		return nil, err
	}
	return []byte(builder.String()), nil
}

// ConversationMeta carries the curated header fields for a conversation thread.
type ConversationMeta struct {
	ConversationID string
	RunID          string
	Topic          string
	State          string
	OpenedAt       string
	ClosedAt       string
}

// ConversationTurn is one curated turn: Speaker (author_session_id) and the Body.
type ConversationTurn struct {
	Speaker string
	Body    string
}

// RenderConversation renders a conversation thread as a chat thread.
func RenderConversation(meta ConversationMeta, turns []ConversationTurn) ([]byte, error) {
	tmpl, err := template.ParseFS(embedded, "templates/conversation.html")
	if err != nil {
		return nil, err
	}
	type viewTurn struct {
		Speaker string
		Body    string
	}
	view := struct {
		ConversationMeta
		Turns []viewTurn
	}{ConversationMeta: meta}
	for _, turn := range turns {
		view.Turns = append(view.Turns, viewTurn{
			Speaker: turn.Speaker,
			Body:    turn.Body,
		})
	}
	var builder strings.Builder
	if err := tmpl.Execute(&builder, view); err != nil {
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
