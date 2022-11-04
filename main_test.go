package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

var tmpDir string

func TestMain(m *testing.M) {
	Logger.Level = debugLevel
	tmpDir = createWorkspace()
	extractBundle(bundle, tmpDir)
	code := m.Run()

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			Logger.Warning("unable to remove tmpDir '%s': %v", tmpDir, err)
		}
	}()

	os.Exit(code)
}

func TestLog(t *testing.T) {
	Logger.Level = debugLevel
	args := map[string]interface{}{}

	if !Logger.Debug("test", args) {
		t.Fatalf("failed to call Debug")
	}

	args = map[string]interface{}{"foo": "bar"}
	if !Logger.Debug("test: %v", args) {
		t.Fatalf("failed to call Debug with args %v", args)
	}
}

func TestRenderTemplate(t *testing.T) {
	c := ansibleInventory{Config}
	tmpl, err := template.New("dummy").Parse(`
	Config = {{ .Config }}
	`)
	if err != nil {
		t.Errorf("expected: a template, got: %v", err)
	}

	b, err := renderTemplate(&c, tmpl)
	if err != nil {
		t.Errorf("expected: bytes, got: %v", err)
	}

	if !strings.Contains(string(b), "Config = ") {
		t.Fatalf("expected: Config = , got: %v", b)
	}
}

func TestInventory(t *testing.T) {
	inventory := ""
	paths := strings.Split("test1.yaml,test2.yaml", ",")
	data := []byte("---\n...")

	for i, p := range paths {
		tp := filepath.Join(tmpDir, p)
		if !extractToFile(tp, data, 0o440) {
			t.Errorf("failed to create file '%s'", tp)
			continue
		}

		sfmt := ",%s"
		if i == 0 {
			sfmt = "%s"
		}

		inventory += fmt.Sprintf(sfmt, tp)
		if _, err := checkInventoryStatus(inventory, tmpDir); err != nil {
			t.Fatalf("failed to checkInventoryStatus for '%s'", inventory)
		}
	}
}

//func TestBundle(t *testing.T) {
//	for _, f := range bundleList {
//		if strings.HasSuffix(f, ".yml") {
//			b, err := bundle.ReadFile(f)
//			if err != nil {
//				t.Fatalf("expected: %s, got: %v", f, err)
//			}
//			extractToFile(tmpDir+"/test", b, 0o440)
//		} else {
//			d, err := bundle.ReadDir(f)
//			if err != nil {
//				t.Fatalf("expected: %s extraction, got: %v", d, err)
//			}
//		}
//	}
//}
