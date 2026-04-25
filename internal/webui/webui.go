package webui

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed dist/index.html dist/assets/*
var embeddedDist embed.FS

type Handler struct {
	api   http.Handler
	dist  fs.FS
	files http.Handler
}

func NewHandler(api http.Handler) (*Handler, error) {
	dist, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		return nil, err
	}
	return &Handler{
		api:   api,
		dist:  dist,
		files: http.FileServer(http.FS(dist)),
	}, nil
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1") {
		handler.api.ServeHTTP(w, r)
		return
	}

	cleanPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if cleanPath != "" && cleanPath != "." {
		file, err := handler.dist.Open(cleanPath)
		if err == nil {
			_ = file.Close()
			handler.files.ServeHTTP(w, r)
			return
		}
	}
	handler.serveIndex(w, r)
}

func (handler *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	content, err := fs.ReadFile(handler.dist, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(content))
}
