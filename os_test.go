package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func generateDummyTarball() ([]byte, error) {
	var buf bytes.Buffer
	var files = []struct {
		Name, Body string
		Mode       int64
	}{
		{Name: "pax_global_header", Body: "Injected header file", Mode: 0o600},
		{Name: "../foo.txt", Body: "Ascending the current directory", Mode: 0o600},
		{Name: "todo.txt", Body: "A file to extract", Mode: 0o600},
	}
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: file.Mode,
			Size: int64(len(file.Body)),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}

		if _, err := tw.Write([]byte(file.Body)); err != nil {
			return nil, err
		}
	}

	if err := gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestGenerateDefaults(t *testing.T) {
	inventory := filepath.Join(tmpDir, "temp-inventory.yaml")

	generateDefaults(inventory)

	if _, err := os.ReadFile(inventory); err != nil {
		t.Fatalf("expected: content from %s, got: %v", inventory, err)
	}
}

func TestExtraction(t *testing.T) {
	b, err := generateDummyTarball()
	if err != nil {
		t.Fatalf("failed to generateDummyTarball: %v", err)
	}

	if !extractBundle([]byte(b), tmpDir) {
		t.Fatalf("failed to extractBundle")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "todo.txt")); err != nil {
		t.Fatalf("failed to extractBundle")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "pax_global_header")); err == nil {
		t.Fatalf("found pax_global_header")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "../foo.txt")); err == nil {
		t.Fatalf("found foo.txt")
	}
}
