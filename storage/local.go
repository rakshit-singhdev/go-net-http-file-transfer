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
) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if key == "" ||
		key != filepath.Base(key) ||
		key == "." ||
		key == ".." ||
		strings.ContainsAny(key, `/\`) {
		return 0, errors.New("invalid storage key")
	}

	path := filepath.Join(s.root, key)
	file, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create stored file: %w", err)
	}
	defer file.Close()

	size, err := io.Copy(file, src)
	if err != nil {
		_ = os.Remove(path)
		return 0, fmt.Errorf("write stored file: %w", err)
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return 0, fmt.Errorf("close stored file: %w", err)
	}

	return size, nil
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" || key != filepath.Base(key) || key == "." || key == ".." ||
		strings.ContainsAny(key, `/\`) {
		return errors.New("invalid storage key")
	}

	if err := os.Remove(filepath.Join(s.root, key)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("delete stored file: %w", err)
	}
	return nil
}
