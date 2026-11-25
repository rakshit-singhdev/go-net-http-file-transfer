package handlers

import (
    "encoding/json"
    "log"
    "net/http"
    "strconv"
)

// List godoc
//
//	@Summary	List files
//	@Tags		files
//	@Produce	json
//	@Param		page	query		int	false	"Page number (1-based)"			default(1)	minimum(1)
//	@Param		limit	query		int	false	"Items per page (max 100)"	default(50)	minimum(1)	maximum(100)
//	@Success	200		{object}	ListResponse
//	@Failure	500		{string}	string	"Unable to retrieve files"
//	@Router		/api/files/ [get]
func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
    limit := int64(50)
    page := int64(1)

    if value := r.URL.Query().Get("limit"); value != "" {
        parsed, err := strconv.ParseInt(value, 10, 64)
        if err == nil && parsed > 0 {
            limit = parsed
        }
    }

    if limit > 100 {
        limit = 100
    }

    if value := r.URL.Query().Get("page"); value != "" {
        parsed, err := strconv.ParseInt(value, 10, 64)
        if err == nil && parsed > 0 && parsed <= 1_000_000 {
            page = parsed
        }
    }

    skip := (page - 1) * limit

    files, err := h.fileServices.ListFiles(
        r.Context(),
        limit,
        skip,
    )

    if err != nil {
        log.Println("List files error:", err)
        http.Error(w, "Unable to retrieve files", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")

    response := ListResponse{
        Files: files,
        Pagination: Pagination{
            Page:  page,
            Limit: limit,
            Count: len(files),
        },
    }

    if err := json.NewEncoder(w).Encode(response); err != nil {
        log.Println("List response encoding error:", err)
    }
}