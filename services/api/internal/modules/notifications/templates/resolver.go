package templates

import (
	"context"
	"fmt"
)

// Resolver renders notification content, preferring DB templates when they exist
// and falling back to inline templates defined in templates.go.
type Resolver interface {
	Render(typ string, data TemplateData) (RenderResult, error)
}

// DBReader reads a template from the database.
type DBReader interface {
	GetDefaultTemplate(ctx context.Context, typ, channel string) (DBTemplate, error)
}

// DBTemplate is the shape returned from the DB query.
type DBTemplate struct {
	Subject  string
	HTMLBody string
}

// dbResolver resolves templates: DB override → inline fallback.
type dbResolver struct {
	reader DBReader
}

// NewResolver creates a Resolver that checks the DB first, then falls back to inline templates.
// If reader is nil, it behaves as a pure inline renderer.
func NewResolver(reader DBReader) Resolver {
	return &dbResolver{reader: reader}
}

func (r *dbResolver) Render(typ string, data TemplateData) (RenderResult, error) {
	if r.reader != nil {
		tmpl, err := r.reader.GetDefaultTemplate(context.Background(), typ, "email")
		if err == nil && tmpl.Subject != "" {
			subj, sErr := renderString(tmpl.Subject, data)
			if sErr != nil {
				return RenderResult{}, fmt.Errorf("render DB subject for %s: %w", typ, sErr)
			}
			body, bErr := renderString(tmpl.HTMLBody, data)
			if bErr != nil {
				return RenderResult{}, fmt.Errorf("render DB html for %s: %w", typ, bErr)
			}
			return RenderResult{Subject: subj, HTMLBody: body, TextBody: stripHTML(body)}, nil
		}
		// DB template not found or error — fall through to inline.
	}
	return Render(typ, data)
}
