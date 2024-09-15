package routes

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"towd/src-server/utils"
)

func SPA(muxer *http.ServeMux, as *utils.AppState, sc chan<- os.Signal) {
	files := http.FS(os.DirFS(as.Config.GetStaticWebClientDir()))
	indexFile, err := files.Open("index.html")
	if err != nil {
		slog.Error("Can't open index.html", "err", err)
		sc <- syscall.SIGTERM
	}
	indexFileStat, err := indexFile.Stat()
	if err != nil {
		slog.Error("Can't get index.html stat", "err", err)
		sc <- syscall.SIGTERM
	}

	muxer.HandleFunc("GET /{filepath...}", func(w http.ResponseWriter, r *http.Request) {
		filepath := filepath.Clean(r.PathValue("filepath"))
		if filepath == "." {
			filepath = "index.html"
		}

		// serve index.html if filepath not found
		file, err := files.Open(filepath)
		if err != nil {
			http.ServeContent(w, r, indexFileStat.Name(), indexFileStat.ModTime(), indexFile)
			return
		}

		// serve index.html if can't get file stat
		stat, err := file.Stat()
		if err != nil {
			http.ServeContent(w, r, indexFileStat.Name(), indexFileStat.ModTime(), indexFile)
			return
		}

		http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
	})
}
