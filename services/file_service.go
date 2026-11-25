package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"file-sharing-server/models"
	"file-sharing-server/repositories"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidFileName = errors.New("invalid file name")
	ErrInvalidExpiry   = errors.New("invalid file expiry")
)

type FileStorage interface {
	Save(ctx context.Context, key string, src io.Reader) (int64, error)
	Open(ctx context.Context, key string) (io.ReadSeekCloser, error)
	Delete(ctx context.Context, key string) error
}

type FileCache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
}

type FileService struct {
    repository *repositories.FileRepository
    storage    FileStorage
    cache      FileCache
    fileTTL    time.Duration
}

func NewFileService(
    repository *repositories.FileRepository,
    storage FileStorage,
    cache FileCache,
    fileTTL time.Duration,
) *FileService {
    return &FileService{
        repository: repository,
        storage:    storage,
        cache:      cache,
        fileTTL:    fileTTL,
    }
}

type UploadInput struct {
	Name        string
	ContentType string
	Content     io.Reader
}

func (s *FileService) Upload(
	ctx context.Context,
	input UploadInput,
) (*models.File, error) {
	name := strings.TrimSpace(input.Name)
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)

	if name == "" || name == "." || name == "/" {
		return nil, ErrInvalidFileName
	}

	if s.fileTTL <= 0 {
		return nil, ErrInvalidExpiry
	}

	if input.Content == nil {
		return nil, errors.New("file content is required")
	}

	now := time.Now().UTC()

	publicID, err := generatePublicID(name, now)
	if err != nil {
		return nil, fmt.Errorf("generate public ID: %w", err)
	}

	storageKey := publicID

	// Save the actual file contents.
	size, err := s.storage.Save(ctx, storageKey, input.Content)
	if err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}

	// Prepare file metadata.
	file := models.File{
		PublicID:     publicID,
		OriginalName: name,
		StorageKey:   storageKey,
		ContentType:  input.ContentType,
		Size:         size,
		CreatedAt:    now,
		ExpiresAt:    now.Add(s.fileTTL),
	}

	// Save metadata in MongoDB.
	if err := s.repository.Create(ctx, file); err != nil {
		// Roll back the stored file if metadata insertion fails.
		if cleanupErr := s.storage.Delete(
			context.Background(),
			storageKey,
		); cleanupErr != nil {
			return nil, fmt.Errorf(
				"save metadata: %w; cleanup failed: %v",
				err,
				cleanupErr,
			)
		}

		return nil, fmt.Errorf("save metadata: %w", err)
	}

	return &file, nil
}

var nonAlphaNumeric = regexp.MustCompile(`[^a-z0-9]+`)

func generatePublicID(name string, now time.Time) (string, error) {
    // Remove the file extension.
    nameWithoutExtension := strings.TrimSuffix(name, path.Ext(name))

    // Convert the filename into a URL-friendly slug.
    slug := strings.ToLower(nameWithoutExtension)
    slug = nonAlphaNumeric.ReplaceAllString(slug, "-")
    slug = strings.Trim(slug, "-")

    if slug == "" {
        slug = "file"
    }

    // Limit the slug length.
    if len(slug) > 40 {
        slug = strings.TrimRight(slug[:40], "-")
    }

    // Generate a random suffix.
    randomBytes := make([]byte, 6)

    if _, err := rand.Read(randomBytes); err != nil {
        return "", err
    }

    randomID := hex.EncodeToString(randomBytes)
    timestamp := now.Format("20060102-150405")

    return fmt.Sprintf("%s-%s-%s", slug, timestamp, randomID), nil
}

var ErrFileNotFound = errors.New("file not found")

func (s *FileService) OpenDownload(
	ctx context.Context,
	publicID string,
) (*models.File, io.ReadSeekCloser, error) {

	file, err := s.GetMetadata(ctx, publicID)
	if err != nil {
		return nil, nil, err
	}

	content, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, ErrFileNotFound
		}

		return nil, nil, fmt.Errorf("open file for download: %w", err)
	}

	return file, content, nil
}

func (s *FileService) ListFiles(
    ctx context.Context,
    limit int64,
    skip int64,
) ([]models.File, error) {

    files, err := s.repository.List(ctx, limit, skip)
    if err != nil {
        return nil, fmt.Errorf("list files: %w", err)
    }

    return files, nil
}
// DeleteFile removes a file's metadata and its stored content.
// Expired files can still be deleted so they can be cleaned up.
func (s *FileService) DeleteFile(ctx context.Context, publicID string) error {
	file, err := s.repository.FindByPublicID(ctx, publicID)
	if err != nil {
		return fmt.Errorf("find file metadata: %w", err)
	}

	if file == nil {
		return ErrFileNotFound
	}

	// Delete metadata first so the file stops being visible even if
	// removing the stored content fails afterwards.
	deleted, err := s.repository.DeleteByPublicID(ctx, publicID)
	if err != nil {
		return fmt.Errorf("delete file metadata: %w", err)
	}

	if !deleted {
		return ErrFileNotFound
	}

	// Drop cached metadata so the deleted file is not served from Redis.
	if s.cache != nil {
		_ = s.cache.Delete(ctx, fileCacheKey(publicID))
	}

	if err := s.storage.Delete(ctx, file.StorageKey); err != nil {
		return fmt.Errorf("delete stored file: %w", err)
	}

	return nil
}

func fileCacheKey(publicID string) string {
    return "files:metadata:" + publicID
}

func (s *FileService) GetMetadata(
    ctx context.Context,
    publicID string,
) (*models.File, error) {

    key := fileCacheKey(publicID)

    // 1. Try Redis first.
    if s.cache != nil {
        cached, err := s.cache.Get(ctx, key)

        if err == nil {
            var file models.File

            if json.Unmarshal(cached, &file) == nil &&
                file.PublicID == publicID &&
                time.Now().UTC().Before(file.ExpiresAt) {
                return &file, nil
            }

            // Invalid or expired cached data.
            _ = s.cache.Delete(ctx, key)
        }

        // On a cache miss or Redis failure, fall back to MongoDB.
    }

    // 2. Cache miss: fetch from MongoDB.
    file, err := s.repository.FindByPublicID(ctx, publicID)
    if err != nil {
        return nil, fmt.Errorf("find file metadata: %w", err)
    }

    if file == nil || !time.Now().UTC().Before(file.ExpiresAt) {
        return nil, ErrFileNotFound
    }

    // 3. Store metadata in Redis.
    if s.cache != nil {
        data, err := json.Marshal(file)

        if err == nil {
            ttl := 5 * time.Minute
            remaining := time.Until(file.ExpiresAt)

            if remaining < ttl {
                ttl = remaining
            }

            if ttl > 0 {
                _ = s.cache.Set(ctx, key, data, ttl)
            }
        }
    }

    return file, nil
}
// ApplyTTLToAllFiles makes the current TTL apply to files uploaded
// before it was changed, by recomputing each file's expiry from its
// creation time.
func (s *FileService) ApplyTTLToAllFiles(ctx context.Context) (int64, error) {
	if s.fileTTL <= 0 {
		return 0, ErrInvalidExpiry
	}

	updated, err := s.repository.ApplyTTL(ctx, s.fileTTL)
	if err != nil {
		return 0, fmt.Errorf("apply file TTL: %w", err)
	}

	return updated, nil
}
