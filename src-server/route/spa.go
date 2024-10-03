package route

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"towd/src-server/utils"
)

func SPA(muxer *http.ServeMux, as *utils.AppState) {
	files := http.FS(os.DirFS(as.Config.GetStaticWebClientDir()))
	indexFile, err := files.Open("index.html")
	if err != nil {
		slog.Error("Can't open index.html", "err", err)
		// as.AppCloseSignalChan <- syscall.SIGTERM
		return
	}
	indexFileStat, err := indexFile.Stat()
	if err != nil {
		slog.Error("Can't get index.html stat", "err", err)
		// as.AppCloseSignalChan <- syscall.SIGTERM
		return
	}

	muxer.HandleFunc("GET /{filepath...}", func(w http.ResponseWriter, r *http.Request) {
		filepath := filepath.Clean(r.PathValue("filepath"))
		switch filepath {
		case ".":
			filepath = "index.html"
		case "calendar":
			filepath = "calendar/index.html"
		case "kanban":
			filepath = "kanban/index.html"
		case "200":
			filepath = "200.html"
		case "404":
			filepath = "404.html"
		}

		file, err := files.Open(filepath)
		if err != nil {
			http.ServeContent(w, r, indexFileStat.Name(), indexFileStat.ModTime(), indexFile)
			return
		}

		stat, err := file.Stat()
		if err != nil {
			http.ServeContent(w, r, indexFileStat.Name(), indexFileStat.ModTime(), indexFile)
			return
		}

		http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
	})
}
