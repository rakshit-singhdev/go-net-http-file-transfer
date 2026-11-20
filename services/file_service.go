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
	"path"
	"strings"
	"time"
)

var (
	ErrInvalidFileName = errors.New("invalid file name")
	ErrInvalidExpiry   = errors.New("invalid file expiry")
)

type FileStorage interface {
	Save(ctx context.Context, key string, src io.Reader) (int64, error)
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

	publicID, err := generatePublicID()
	if err != nil {
		return nil, fmt.Errorf("generate public ID: %w", err)
	}

	// Use the public ID as the internal storage key.
	storageKey := publicID

	// Save the actual file contents.
	size, err := s.storage.Save(ctx, storageKey, input.Content)
	if err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}

	now := time.Now().UTC()

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

func generatePublicID() (string, error) {
	bytes := make([]byte, 16)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
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

	if time.Now().UTC().After(file.ExpiresAt){
		return nil, ErrFileNotFound
	}
	
	return file, nil
}