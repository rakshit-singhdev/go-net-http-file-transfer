package handlers

import (
	"encoding/json"
	"errors"
	"file-sharing-server/services"
	"fmt"
	"net/http"
	"path/filepath"
)

type FileHandler struct {
	fileServices  *services.FileService
	maxUploadSize int64
}

func NewFileHandler(
	fileServices *services.FileService,
	maxUploadSize int64,
) *FileHandler {
	return &FileHandler{
		fileServices:  fileServices,
		maxUploadSize: maxUploadSize,
	}
}

func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var maxBytesErr *http.MaxBytesError

		if errors.As(err, &maxBytesErr) {
			http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
			return
		}

		http.Error(w, "Invalid multipart request", http.StatusBadRequest)
		return
	}

	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, header, err := r.FormFile("file")

	if err != nil {
		http.Error(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	contentType := http.DetectContentType(buffer[:n])

	if _, err := file.Seek(0, 0); err != nil {
		http.Error(w, "Unable to read file", http.StatusInternalServerError)
		return
	}

	uploadedFile, err := h.fileServices.Upload(
		r.Context(),
		services.UploadInput{
			Name:        filepath.Base(header.Filename),
			ContentType: contentType,
			Content:     file,
		},
	)
	if err != nil {
		fmt.Println("Upload error:", err)
		http.Error(w, "Unable to upload file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	response := map[string]any{
		"message": "File uploaded successfully",
		"file":    uploadedFile,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Println("Response encoding error:", err)
	}

}
