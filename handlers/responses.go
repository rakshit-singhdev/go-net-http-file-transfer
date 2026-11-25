package handlers

import "file-sharing-server/models"

// UploadResponse is returned after a successful upload.
type UploadResponse struct {
	Message string       `json:"message" example:"File uploaded successfully"`
	File    *models.File `json:"file"`
}

// MetadataResponse wraps a file's metadata.
type MetadataResponse struct {
	File *models.File `json:"file"`
}
