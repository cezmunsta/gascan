package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateDefaults(t *testing.T) {
	inventory := filepath.Join(tmpDir, "temp-inventory.yaml")

	generateDefaults(inventory)

	_, err := os.ReadFile(inventory)

	if err != nil {
		t.Fatalf("expected: content from %s, got: %v", inventory, err)
	}
}
