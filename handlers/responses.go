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

// Pagination describes the page returned by a list request.
type Pagination struct {
	Page  int64 `json:"page" example:"1"`
	Limit int64 `json:"limit" example:"50"`
	Count int   `json:"count" example:"2"`
}

// ListResponse is returned when listing files.
type ListResponse struct {
	Files      []models.File `json:"files"`
	Pagination Pagination    `json:"pagination"`
}
