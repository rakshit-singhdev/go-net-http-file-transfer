package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalStorage struct {
	root string
}

func NewLocalStorage(root string) (*LocalStorage, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("storge director is required")
	}

	absPath, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve storage directory: %w", err)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	return &LocalStorage{
		root: absPath,
	}, nil
}

func (s *LocalStorage) Save(
	ctx context.Context,
	key string,
	src io.Reader,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if key == "" ||
		key != filepath.Base(key) ||
		key == "." ||
		key == ".." ||
		strings.ContainsAny(key, `/\`) {
		return errors.New("invalid storage key")
	}

	err := os.Remove(filepath.Join(s.root, key))
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Delete stored file: %w", err)
	}

	return nil
}
