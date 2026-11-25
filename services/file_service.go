package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

type FileService struct {
	repository *repositories.FileRepository
	storage    FileStorage
	fileTTL    time.Duration
}

func NewFileService(
	repository *repositories.FileRepository,
	storage FileStorage,
	fileTTL time.Duration,
) *FileService {
	return &FileService{
		repository: repository,
		storage:    storage,
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

func (s *FileService) GetMetadata(ctx context.Context, publicID string) (*models.File, error) {
	file, err := s.repository.FindByPublicID(ctx, publicID)
	if err != nil {
		return nil, fmt.Errorf("find file metadata: %w", err)
	}

	if file == nil {
		return nil, ErrFileNotFound
	}

	if time.Now().UTC().After(file.ExpiresAt) {
		return nil, ErrFileNotFound
	}

	return file, nil
}

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

	if err := s.storage.Delete(ctx, file.StorageKey); err != nil {
		return fmt.Errorf("delete stored file: %w", err)
	}

	return nil
}
