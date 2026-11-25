package handlers

import (
    "encoding/json"
    "errors"
    "net/http"

    "file-sharing-server/services"
)

// GetMetadata godoc
//
//	@Summary	Get file metadata
//	@Tags		files
//	@Produce	json
//	@Param		id	path		string	true	"File ID"
//	@Success	200	{object}	MetadataResponse
//	@Failure	404	{string}	string	"File not found"
//	@Failure	500	{string}	string	"Unable to retrieve file metadata"
//	@Router		/api/files/{id} [get]
func (h *FileHandler) GetMetadata(
    w http.ResponseWriter,
    r *http.Request,
) {
    id := r.PathValue("id")

    file, err := h.fileServices.GetMetadata(r.Context(), id)
    if err != nil {
        if errors.Is(err, services.ErrFileNotFound) {
            http.Error(w, "File not found", http.StatusNotFound)
            return
        }

        http.Error(w, "Unable to retrieve file metadata", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")

    if err := json.NewEncoder(w).Encode(MetadataResponse{
        File: file,
    }); err != nil {
        return
    }
}