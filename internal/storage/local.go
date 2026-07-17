package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	dataRoot string
}

func NewLocalStorage(dataRoot string) *LocalStorage {
	return &LocalStorage{
		dataRoot: dataRoot,
	}
}

func (s *LocalStorage) Init() error {
	err := os.MkdirAll(s.dataRoot, 0o750)
	if err != nil {
		return fmt.Errorf("init data dir %s: %w", s.dataRoot, err)
	}

	return nil
}

func (s *LocalStorage) Get(key string, value any) error {
	path := s.formatPath(key)

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("get key %s: %w", key, err)
	}

	err = json.Unmarshal(content, value)
	if err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}

	return nil
}

func (s *LocalStorage) Put(key string, value any) error {
	path := s.formatPath(key)

	content, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	_, err = tmpFile.Write(content)
	if err != nil {
		os.Remove(tmpFile.Name())
		return fmt.Errorf("write tmp file: %w", err)
	}

	err = os.Rename(tmpFile.Name(), path)
	if err != nil {
		return fmt.Errorf("rename tmp file: %w", err)
	}

	return nil
}

func (s *LocalStorage) Delete(key string) error {
	path := s.formatPath(key)

	err := os.Remove(path)
	if err != nil {
		return fmt.Errorf("delete key %s: %w", key, err)
	}

	return nil
}

func (s *LocalStorage) formatPath(key string) string {
	return fmt.Sprintf("%s/%s", s.dataRoot, key)
}
