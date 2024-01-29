package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

func cleanupWorkspace(path string) error {
	if err := os.RemoveAll(path); err != nil {
		Logger.Warning("unable to remove tmpDir '%s': %v", path, err)
		return err
	}

	return nil
}

func createWorkspace() string {
	tmpDir, err := os.MkdirTemp(Config.ExtractPath, "onboarding")
	if err != nil {
		Logger.Fatal("failed to create tmpDir '%s'; %v", tmpDir, err)
	}

	return tmpDir
}

// RunPlaybook via ansible-playbook
func RunPlaybook(ansibleConfig string, args ...string) (bool, int) {
	c := generateCommand(Ansible, args...)
	c.Env = append(os.Environ(), "PEX_SCRIPT=ansible-playbook", fmt.Sprintf("ANSIBLE_CONFIG=%s", ansibleConfig))

	Logger.Debug("Executing playbook: %s", c.Env)

	if err := c.Run(); err != nil {
		Logger.Error("failed to execute command '%s': %s", c, err)
		return false, c.ProcessState.ExitCode()
	}

	return true, 0
}

func editTemplates(path string) error {
	args := append([]string{Config.Editor}, strings.Split(path, " ")...)
	c := generateCommand("command", args...)

	Logger.Debug("configuring templates %s", path)

	if err := c.Run(); err != nil {
		Logger.Error("failed to execute command '%s': %s", c, err)
		return err
	}

	return nil
}

func generateDefaults(inventory string) {
	d := path.Dir(inventory)
	t := []string{filepath.Join(d, defaultInventory), filepath.Join(d, defaultConfig)}

	for _, tmplSrc := range t {
		j2, err := os.ReadFile(tmplSrc)
		if err != nil {
			Logger.Fatal("unable to load template '%s'", tmplSrc)
		}

		tmpl, err := template.New(tmplSrc).Parse(string(j2))
		if err != nil {
			Logger.Fatal("failed to parse template '%s': %v", tmplSrc, err)
		}

		switch path.Base(tmplSrc) {
		case path.Base(defaultInventory):
			ds := ansibleInventory{Config}
			content, _ := renderTemplate(&ds, tmpl)
			extractToFile(inventory, content, 0o440)
		}
	}
}

func extractToFile(path string, content []byte, mode fs.FileMode) bool {
	Logger.Debug("extracting %s", path)

	err := os.WriteFile(path, content, mode)
	if err != nil {
		Logger.Fatal("failed to extract file to disk '%s': %s", path, err)
	}

	return true
}

func extractBundle(tgz []byte, targetDir string) bool {
	var mode fs.FileMode

	buf := bytes.NewBuffer(tgz)
	gbuf, err := gzip.NewReader(buf)
	if err != nil {
		Logger.Fatal("unable to read with gzip: %v", err)
	}

	tbuf := bytes.Buffer{}
	tr := tar.NewReader(gbuf)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			Logger.Fatal("failed to extract tarball: %v", err)
		}

		// Fail if an unexpected prefix exists, or the path ascends the directory tree
		if strings.Contains(hdr.Name, "..") || strings.Contains(hdr.Linkname, "..") {
			Logger.Warning("unexpected path found during extraction: %v", hdr.Name)
			continue
		}

		pth := filepath.Join(targetDir, strings.Replace(hdr.Name, "automation/", "", 1))

		switch hdr.Typeflag {
		case tar.TypeDir:
			Logger.Debug("Directory = %s", pth)
			if err := os.MkdirAll(pth, 0o750); err != nil {
				Logger.Fatal("failed to create directory: %v", err)
			}
		case tar.TypeReg:
			Logger.Debug("File = %s", pth)
			if _, err := io.Copy(&tbuf, tr); err == nil {
				switch strings.Replace(pth, targetDir, "", 1) {
				case "/pax_global_header":
					tbuf.Reset()
					Logger.Debug("ignoring %s", pth)
					continue
				default:
					mode = 0o440
				}
				Logger.Debug("extracting %s", pth)
				extractToFile(pth, tbuf.Bytes(), mode)
				tbuf.Reset()
			}
		case tar.TypeSymlink:
			Logger.Debug("Symlink = %s", pth)
			linkname := strings.Replace(hdr.Linkname, "automation/", "", 1)
			if err := os.Symlink(linkname, pth); err != nil {
				Logger.Fatal("failed to create symlink: %v", err)
			}

		}
	}

	return true
}
