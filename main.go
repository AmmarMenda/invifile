package main

import (
	"archive/zip"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/disintegration/imaging"

	_ "golang.org/x/image/webp"
)

var baseDir string

//go:embed static/* face/*
var embeddedFiles embed.FS

type FileInfo struct {
	Name    string
	IsDir   bool
	Path    string
	IsImage bool
	Size    int64 // Added
	ModTime int64 // Added (Unix timestamp)
	Ext     string
}

type PageData struct {
	CurrentPath string
	ParentPath  string
	Files       []FileInfo
	DiskPercent float64 // Added
	DiskLabel   string
}

func zipHandler(w http.ResponseWriter, r *http.Request) {
	// Get paths from the query string
	paths := r.URL.Query()["p"]
	if len(paths) == 0 {
		http.Error(w, "No files selected", 400)
		return
	}

	// Set headers so the browser knows it's a file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=invifiles_bundle.zip")

	archive := zip.NewWriter(w)
	defer archive.Close()

	for _, relPath := range paths {
		fullPath := filepath.Join(baseDir, relPath)

		// Skip if it's a directory (keeping it simple for now)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}

		file, _ := os.Open(fullPath)

		// Create a entry in the zip
		f, _ := archive.Create(filepath.Base(relPath))
		io.Copy(f, file)
		file.Close()
	}
}

func getDiskUsage(path string) (float64, string) {
	var stat syscall.Statfs_t
	wd, _ := filepath.Abs(path)
	syscall.Statfs(wd, &stat)

	// Available blocks * size per block
	all := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := all - free

	percent := (float64(used) / float64(all)) * 100

	// Convert bytes to GB for the label
	gbUsed := float64(used) / (1024 * 1024 * 1024)
	gbTotal := float64(all) / (1024 * 1024 * 1024)

	return percent, fmt.Sprintf("%.1fGB / %.1fGB", gbUsed, gbTotal)
}

func main() {
	baseDir = "."
	if len(os.Args) > 1 {
		baseDir = os.Args[1]
	}
	baseDir, _ = filepath.Abs(baseDir)

	// --- NEW: Serve static files from the binary ---
	staticFS, _ := fs.Sub(embeddedFiles, "static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	http.HandleFunc("/view/", listHandler)
	http.Handle("/download/", http.StripPrefix("/download/", http.FileServer(http.Dir(baseDir))))
	http.HandleFunc("/thumb/", thumbHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/view/", http.StatusSeeOther)
	})

	fmt.Printf(":) Serving %s on http://localhost:9000\n", baseDir)
	http.ListenAndServe(":9000", nil)
}

func thumbHandler(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/thumb")
	fullPath := filepath.Join(baseDir, relPath)

	src, err := imaging.Open(fullPath)
	if err != nil {
		http.Error(w, "File not found", 404)
		return
	}

	dst := imaging.Fill(src, 150, 150, imaging.Center, imaging.Lanczos)

	w.Header().Set("Content-Type", "image/jpeg")
	imaging.Encode(w, dst, imaging.JPEG)
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/view")
	if relPath == "" {
		relPath = "/"
	}

	fullPath := filepath.Join(baseDir, relPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "Directory not found", 404)
		return
	}

	var files []FileInfo
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		isImg := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" || ext == ".gif"
		info, _ := entry.Info()
		files = append(files, FileInfo{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Path:    filepath.Join(relPath, entry.Name()),
			IsImage: isImg,
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
			Ext:     strings.ToLower(filepath.Ext(entry.Name())),
		})
	}
	percent, label := getDiskUsage(baseDir)
	data := PageData{
		CurrentPath: relPath,
		ParentPath:  filepath.Dir(strings.TrimSuffix(relPath, "/")),
		Files:       files,
		DiskPercent: percent,
		DiskLabel:   label,
	}

	t, err := template.ParseFS(embeddedFiles, "face/index.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	t.Execute(w, data)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}
	targetRelPath := r.URL.Query().Get("path")
	file, header, _ := r.FormFile("myFile")
	defer file.Close()
	outPath := filepath.Join(baseDir, targetRelPath, header.Filename)
	out, _ := os.Create(outPath)
	defer out.Close()
	io.Copy(out, file)
	http.Redirect(w, r, "/view/"+targetRelPath, http.StatusSeeOther)
}
