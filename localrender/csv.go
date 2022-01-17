package localrender

import (
	"io"

	"github.com/gobuffalo/buffalo/render"
)

type templateDelegatedRenderer struct {
	delegate    render.Renderer
	contentType string
}

func (t templateDelegatedRenderer) ContentType() string {
	return t.contentType
}

func (s *templateDelegatedRenderer) Render(w io.Writer, data render.Data) error {
	return s.delegate.Render(w, data)
}

// CSV renders the named files using the render.Plain but changing content type to csv
func Csv(e *render.Engine, names ...string) render.Renderer {
	return &templateDelegatedRenderer{
		delegate:    e.Plain(names...),
		contentType: "text/csv;  charset=utf-8",
	}
}
