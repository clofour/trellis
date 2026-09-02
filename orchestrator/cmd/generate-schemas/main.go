// Command generate-schemas regenerates the checked-in Trellis job schemas.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/clofour/trellis/internal/specschema"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	check := flag.Bool("check", false, "fail if the checked-in schemas differ from generated output")
	outputDir := flag.String("output-dir", filepath.Join("..", "schemas"), "directory containing generated schema files")
	flag.Parse()

	apiSchema, yamlSchema, err := specschema.Generate()
	if err != nil {
		return err
	}
	files := []struct {
		name string
		data []byte
	}{
		{name: "trellis-job-api.schema.json", data: apiSchema},
		{name: "trellis-job.schema.json", data: yamlSchema},
	}

	if *check {
		var stale []string
		for _, file := range files {
			path := filepath.Join(*outputDir, file.name)
			existing, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(existing, file.data) {
				stale = append(stale, file.name)
			}
		}
		if len(stale) > 0 {
			return fmt.Errorf("generated schemas are stale: %v; run 'go run ./cmd/generate-schemas' from orchestrator/", stale)
		}
		return nil
	}

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		return fmt.Errorf("create schema directory: %w", err)
	}
	for _, file := range files {
		path := filepath.Join(*outputDir, file.name)
		if err := os.WriteFile(path, file.data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Println(path)
	}
	return nil
}
