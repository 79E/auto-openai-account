package webui

import (
	"net/http"
)

func Handler() http.Handler {
	return spaHandler{fs: http.Dir("dist")}
}

type spaHandler struct{ fs http.FileSystem }

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}
	file, err := h.fs.Open(path[1:])
	if err != nil {
		r.URL.Path = "/index.html"
		http.FileServer(h.fs).ServeHTTP(w, r)
		return
	}
	_ = file.Close()
	http.FileServer(h.fs).ServeHTTP(w, r)
}
