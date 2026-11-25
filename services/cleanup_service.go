package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

func (s *FileService) CleanupExpiredFiles(
    ctx context.Context,
) (int, error) {

    files, err := s.repository.ListExpired(
        ctx,
        time.Now().UTC(),
        100,
    )

    if err != nil {
        return 0, fmt.Errorf("find expired files: %w", err)
    }

    deletedCount := 0
    var cleanupErrors []error

    for _, file := range files {
        if err := ctx.Err(); err != nil {
            cleanupErrors = append(cleanupErrors, err)
            break
        }

        // Delete physical file first.
        if err := s.storage.Delete(ctx, file.StorageKey); err != nil {
            cleanupErrors = append(
                cleanupErrors,
                fmt.Errorf("%s: delete physical file: %w", file.PublicID, err),
            )
            continue
        }

        // Delete MongoDB metadata.
        deleted, err := s.repository.DeleteByPublicID(ctx, file.PublicID)
        if err != nil {
            cleanupErrors = append(
                cleanupErrors,
                fmt.Errorf("%s: delete metadata: %w", file.PublicID, err),
            )
            continue
        }

        if deleted {
            deletedCount++
        }
    }

    return deletedCount, errors.Join(cleanupErrors...)
}

func RunCleanupWorker(
    ctx context.Context,
    fileService *FileService,
    interval time.Duration,
) {
    if interval <= 0 {
        interval = time.Minute
    }

    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    logCleanup := func() {
        cleanupCtx, cancel := context.WithTimeout(
            ctx,
            30*time.Second,
        )
        defer cancel()

        count, err := fileService.CleanupExpiredFiles(cleanupCtx)

        if err != nil {
            log.Println("Cleanup worker error:", err)
            return
        }

        if count > 0 {
            log.Printf("Cleanup worker deleted %d expired files", count)
        }
    }

    // Run once immediately.
    logCleanup()

    for {
        select {
        case <-ctx.Done():
            log.Println("Cleanup worker stopped")
            return

        case <-ticker.C:
            logCleanup()
        }
    }
}