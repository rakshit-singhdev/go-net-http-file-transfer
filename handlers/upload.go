package handlers

import (
	"encoding/json"
	"errors"
	"file-sharing-server/middleware"
	"file-sharing-server/services"
	"fmt"
	"net/http"
	"path/filepath"

	"go.mongodb.org/mongo-driver/v2/bson"
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

// Upload godoc
//
//	@Summary		Upload a file
//	@Description	Uploads a file via multipart/form-data. The file expires after the configured TTL.
//	@Tags			files
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			file	formData	file	true	"File to upload"
//	@Success		201		{object}	UploadResponse
//	@Failure		400		{string}	string	"Invalid multipart request or file is required"
//	@Failure		413		{string}	string	"File too large"
//	@Failure		500		{string}	string	"Unable to upload file"
//	@Router			/api/files [post]
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)

	ownerID, ok := r.Context().Value(
		middleware.UserIDContextKey,
	).(bson.ObjectID)

	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

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
			OwnerID:     ownerID,
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

	response := UploadResponse{
		Message: "File uploaded successfully",
		File:    uploadedFile,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Println("Response encoding error:", err)
	}

}
