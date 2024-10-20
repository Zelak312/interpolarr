package views

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin/render"
)

// HTMLTemplRenderer implements gin's render.HTMLRender
type HTMLTemplRenderer struct{}

// Instance returns a Render for templ.Component
func (r *HTMLTemplRenderer) Instance(s string, d any) render.Render {
	component, ok := d.(templ.Component)
	if !ok {
		return nil // or handle error as needed
	}
	return &Renderer{Ctx: context.Background(), Component: component}
}

// Renderer for templ.Component
type Renderer struct {
	Ctx       context.Context
	Component templ.Component
}

// Render outputs the templ.Component
func (r Renderer) Render(w http.ResponseWriter) error {
	r.WriteContentType(w) // Call the content type method
	return r.Component.Render(r.Ctx, w)
}

// WriteContentType sets the Content-Type header for the response
func (r Renderer) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}
