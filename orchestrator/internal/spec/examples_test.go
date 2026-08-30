package spec

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestExampleManifestsValidate(t *testing.T) {
	root := filepath.Join("..", "..", "..", "examples")

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		t.Run(rel, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			job, err := ParseYAML(raw)
			if err != nil {
				t.Fatalf("parse example manifest: %v", err)
			}
			if err := Validate(job); err != nil {
				t.Fatalf("validate example manifest: %v", err)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk examples: %v", err)
	}
}
