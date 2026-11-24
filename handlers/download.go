package handlers

import (
    "errors"
    "log"
    "mime"
    "net/http"

    "file-sharing-server/services"
)

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