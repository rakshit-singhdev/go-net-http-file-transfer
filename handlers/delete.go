package handlers

import (
	"errors"
	"log"
	"net/http"

	"file-sharing-server/services"
)

// Delete godoc
//
//	@Summary	Delete a file
//	@Tags		files
//	@Param		id	path	string	true	"File ID"
//	@Success	204	"File deleted"
//	@Failure	404	{string}	string	"File not found"
//	@Failure	500	{string}	string	"Unable to delete file"
//	@Router		/api/files/{id} [delete]
func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.fileServices.DeleteFile(r.Context(), id); err != nil {
		if errors.Is(err, services.ErrFileNotFound) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		log.Println("Delete error:", err)
		http.Error(w, "Unable to delete file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
