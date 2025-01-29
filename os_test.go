package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

const ansibleDummyConfig = `
[defaults]

callback_plugins = ./plugins/callback
callbacks_enabled = inspect

inventory = ~/.config/gascan/inventory-config.json,~/.config/gascan/secrets.yaml
inventory_plugins = ./plugins/inventory

strategy_plugins = ./plugins/strategy
strategy = locking

[inventory]

enable_plugins = dynamic_inventory_plugin,yaml
`

func generateDummyTarball() ([]byte, error) {
	var buf bytes.Buffer
	files := []struct {
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

func TestClearCache(t *testing.T) {
	var err error

	EnvCachePaths = "~,~/"
	cacheDir := ""

	if err = clearInventoryCache(); err == nil {
		t.Fatalf("expected an error for EnvCachePaths %q", EnvCachePaths)
	}

	if cacheDir, err = os.MkdirTemp(os.TempDir(), "cache1"); err != nil {
		t.Fatalf("unable to create directory: %v", err)
	}

	defer os.RemoveAll(cacheDir)

	EnvCachePaths = cacheDir

	if cacheDir, err = os.MkdirTemp(os.TempDir(), "cache2"); err != nil {
		t.Fatalf("unable to create directory: %v", err)
	}

	EnvCachePaths += "," + filepath.Join(cacheDir, "foobar*") + "," + cacheDir

	if err = clearInventoryCache(); err != nil {
		t.Fatalf("unexpected error for EnvCachePaths %q", EnvCachePaths)
	}
}

func TestAnsibleConfig(t *testing.T) {
	cfgDir := ""

	if td, err := os.MkdirTemp(os.TempDir(), "cfg"); err == nil {
		cfgDir = td
	} else {
		t.Fatalf("unable to create directory: %v", err)
	}

	defer os.RemoveAll(cfgDir)

	dummyCfg := filepath.Join(cfgDir, "default.cfg")
	newCfgName := filepath.Join(cfgDir, "new.cfg")

	if err := os.WriteFile(dummyCfg, []byte(ansibleDummyConfig), 0o640); err != nil {
		t.Fatalf("unable to write the dummy config: %v", err)
	}

	if newCfg := makeAbsolutePaths(dummyCfg, "new.cfg"); newCfg != newCfgName {
		t.Fatalf("expected '%v', got '%v'", newCfgName, newCfg)
	}

	dummyCfg = newCfgName

	if newContent, err := os.ReadFile(dummyCfg); err != nil || len(newContent) == 0 {
		t.Fatalf("unable to read the new dummy config: %v", err)
	}
}
