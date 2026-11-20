package handlers

import (
    "encoding/json"
    "errors"
    "net/http"

    "file-sharing-server/services"
)

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

    if err := json.NewEncoder(w).Encode(map[string]any{
        "file": file,
    }); err != nil {
        return
    }
}