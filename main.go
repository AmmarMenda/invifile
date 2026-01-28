package main

import (
	"archive/zip"
	"bufio"
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"
	"time"

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
	IsVideo bool  // Added
	Size    int64 // Added
	ModTime int64 // Added (Unix timestamp)
	Ext     string
}

type PageData struct {
	CurrentPath string
	ParentPath  string
	Files       []FileInfo
	DiskPercent float64
	DiskLabel   string
}

func zipHandler(w http.ResponseWriter, r *http.Request) {
	paths := r.URL.Query()["p"]
	if len(paths) == 0 {
		http.Error(w, "No files selected", 400)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=invifiles_bundle.zip")

	archive := zip.NewWriter(w)
	defer archive.Close()

	for _, relPath := range paths {
		fullPath := filepath.Join(baseDir, relPath)

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

	all := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := all - free

	percent := (float64(used) / float64(all)) * 100

	gbUsed := float64(used) / (1024 * 1024 * 1024)
	gbTotal := float64(all) / (1024 * 1024 * 1024)

	return percent, fmt.Sprintf("%.1fGB / %.1fGB", gbUsed, gbTotal)
}

func thumbHandler(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/thumb")
	fullPath := filepath.Join(baseDir, relPath)

	ext := strings.ToLower(filepath.Ext(fullPath))
	isVid := ext == ".mp4" || ext == ".mov" || ext == ".mkv" || ext == ".webm"

	if isVid {
		if _, err := os.Stat(fullPath); err != nil {
			http.Error(w, "File not found", 404)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		cmd := exec.Command("ffmpeg",
			"-i", fullPath,
			"-ss", "00:00:01",
			"-vframes", "1",
			"-vf", "scale=150:150:force_original_aspect_ratio=increase,crop=150:150",
			"-f", "image2",
			"-",
		)
		cmd.Stdout = w
		if err := cmd.Run(); err != nil {
			fmt.Printf("Thumbnail error for %s: %v\n", relPath, err)
		}
		return
	}

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
		isVid := ext == ".mp4" || ext == ".mov" || ext == ".mkv" || ext == ".webm"
		info, _ := entry.Info()
		files = append(files, FileInfo{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Path:    filepath.Join(relPath, entry.Name()),
			IsImage: isImg,
			IsVideo: isVid,
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

const (
	ExitQuit    = 0
	ExitRestart = 1
)

func main() {
	baseDir = "."
	if len(os.Args) > 1 {
		baseDir = os.Args[1]
	}
	baseDir, _ = filepath.Abs(baseDir)

	for {
		exitCode := runServer(baseDir)
		if exitCode == ExitQuit {
			fmt.Println("bye bye :(")
			break
		}
		fmt.Println("Restarting server...")
		time.Sleep(500 * time.Millisecond)
	}
}

func runServer(dir string) int {
	mux := http.NewServeMux()

	staticFS, _ := fs.Sub(embeddedFiles, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("/view/", listHandler)
	mux.Handle("/download/", http.StripPrefix("/download/", http.FileServer(http.Dir(dir))))
	mux.HandleFunc("/thumb/", thumbHandler)
	mux.HandleFunc("/upload", uploadHandler)
	mux.HandleFunc("/zip", zipHandler)
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("Manual panic triggered")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/view/", http.StatusSeeOther)
	})

	srv := &http.Server{
		Addr:    ":9000",
		Handler: recoverMiddleware(mux),
	}

	serverError := make(chan error, 1)
	go func() {
		fmt.Printf(":) Serving %s on http://localhost:9000\nPress r and hit enter to restart and q to quit", dir)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			serverError <- err
		}
	}()

	cli := make(chan int)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text == "r" {
				cli <- ExitRestart
				return
			}
			if text == "q" {
				cli <- ExitQuit
				return
			}
		}
	}()

	select {
	case code := <-cli:
		srv.Shutdown(context.Background())
		return code
	case err := <-serverError:
		fmt.Printf("Server error: %v\n", err)
		return ExitRestart // Auto-restart on server error? Or maybe panic recovery handles this.
	}
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("Panic detected: %v\n", err)
				fmt.Println("Recovered from panic, signaling restart...")
				http.Error(w, "Server panic recovered", 500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}
	targetRelPath := r.URL.Query().Get("path")

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Redirect(w, r, "/view/"+targetRelPath+"?error=empty", http.StatusSeeOther)
		return
	}

	files := r.MultipartForm.File["myFile"]
	if len(files) == 0 {
		http.Redirect(w, r, "/view/"+targetRelPath+"?error=empty", http.StatusSeeOther)
		return
	}

	for _, header := range files {
		if header.Size == 0 {
			continue
		}
		file, err := header.Open()
		if err != nil {
			continue
		}
		outPath := filepath.Join(baseDir, targetRelPath, header.Filename)
		out, err := os.Create(outPath)
		if err != nil {
			file.Close()
			continue
		}
		io.Copy(out, file)
		file.Close()
		out.Close()
	}

	http.Redirect(w, r, "/view/"+targetRelPath, http.StatusSeeOther)
}
