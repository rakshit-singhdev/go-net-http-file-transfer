package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	// "net/http/httputil"
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON",
		})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Name is required",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Hello " + req.Name,
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()

		next.ServeHTTP(w, r)
		elapsed := time.Since(start)

		log.Println(
			r.Method,
			r.URL.Path,
			elapsed,
		)

	})
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {

    // Enforce maximum request size: 2 MiB
    r.Body = http.MaxBytesReader(w, r.Body, 2<<20)

    // 1. Parse multipart form
    if err := r.ParseMultipartForm(2 << 20); err != nil {
        var maxBytesErr *http.MaxBytesError

        if errors.As(err, &maxBytesErr) {
            writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
                "error": "Request exceeds 2 MiB limit",
            })
            return
        }

        writeJSON(w, http.StatusBadRequest, map[string]string{
            "error": "Invalid multipart form",
        })
        return
    }

	// 2. Retrieve the uploaded file using the field name "file"
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Unable to retrieve file: " + err.Error(),
		})
		return
	}

	// 3. Close the uploaded file when the handler finishes
	defer file.Close() // defer means perform the action after function ends

	// 4. Create an uploads directory if it doesn't exist
	if err := os.MkdirAll("uploads", 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to create upload directory" + err.Error(),
		})
		return
	}

	// 4. Get safe filename
	filename := filepath.Base(header.Filename)

	// 5. Create destination file
	dst, err := os.Create(filepath.Join("uploads", filename))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to create destination file",
		})
		return
	}
	defer dst.Close()

	// 6. Copy uploaded content
	size, err := io.Copy(dst, file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Unable to save file",
		})
		return
	}

	// 7. Return response
	writeJSON(w, http.StatusOK, map[string]any{
		"message":  "File uploaded successfully",
		"filename": filename,
		"size":     size,
	})
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", healthHandler)
	mux.HandleFunc("POST /hello", helloHandler)
	mux.HandleFunc("POST /api/files", uploadHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: loggingMiddleware(mux),
	}
	log.Println("Server running on port 8080")
	log.Fatal(server.ListenAndServe())
}
