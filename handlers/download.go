package handlers

import (
    "errors"
    "log"
    "mime"
    "net/http"

    "file-sharing-server/services"
)

// Download godoc
//
//	@Summary		Download a file
//	@Description	Streams the file content as an attachment. Supports HTTP Range requests.
//	@Tags			files
//	@Produce		octet-stream
//	@Param			id	path		string	true	"File ID"
//	@Success		200	{file}		binary
//	@Failure		404	{string}	string	"File not found"
//	@Failure		500	{string}	string	"Unable to download file"
//	@Router			/api/files/{id}/download [get]
func (h *FileHandler) Download(
    w http.ResponseWriter,
    r *http.Request,
) {
    id := r.PathValue("id")

    file, content, err := h.fileServices.OpenDownload(
        r.Context(),
        id,
    )

    if err != nil {
        if errors.Is(err, services.ErrFileNotFound) {
            http.Error(w, "File not found", http.StatusNotFound)
            return
        }

        log.Println("Download error:", err)
        http.Error(w, "Unable to download file", http.StatusInternalServerError)
        return
    }

    defer content.Close()

    contentType := file.ContentType
    if contentType == "" {
        contentType = "application/octet-stream"
    }

    w.Header().Set("Content-Type", contentType)

    w.Header().Set(
        "Content-Disposition",
        mime.FormatMediaType("attachment", map[string]string{
            "filename": file.OriginalName,
        }),
    )

    http.ServeContent(
        w,
        r,
        file.OriginalName,
        file.CreatedAt,
        content,
    )
}