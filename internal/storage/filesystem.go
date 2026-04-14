package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type FilesystemStorage struct {
	root string
}

func NewFilesystemStorage(root string) *FilesystemStorage {
	return &FilesystemStorage{root: root}
}

func (s *FilesystemStorage) Store(suggestedPath string, data io.Reader, _ int64, _ string) (string, error) {
	target := s.resolve(suggestedPath)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", fmt.Errorf("create artifact dir: %w", err)
	}
	f, err := os.Create(target)
	if err != nil {
		return "", fmt.Errorf("create artifact file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, data); err != nil {
		return "", fmt.Errorf("write artifact: %w", err)
	}
	return suggestedPath, nil
}

func (s *FilesystemStorage) Retrieve(storagePath string) (io.ReadCloser, error) {
	f, err := os.Open(s.resolve(storagePath))
	if err != nil {
		return nil, fmt.Errorf("open artifact: %w", err)
	}
	return f, nil
}

func (s *FilesystemStorage) Delete(storagePath string) error {
	if err := os.Remove(s.resolve(storagePath)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete artifact: %w", err)
	}
	return nil
}

func (s *FilesystemStorage) resolve(p string) string {
	return filepath.Join(s.root, filepath.FromSlash(p))
}
