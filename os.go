package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
)

// EnvCachePaths is a comma-separated set of path names, which can be configured at build time
var EnvCachePaths string = "/tmp/.gascan/inventory.cache,~/.config/gascan/cache/gas_inventory_dynamic_inventory_plugin*"

func cleanupWorkspace(path string) error {
	Logger.Debug("cleaning path %q", path)

	if err := os.RemoveAll(path); err != nil {
		Logger.Warning("unable to remove tmpDir '%s': %v", path, err)
		return err
	}

	return nil
}

func clearInventoryCache() error {
	currentUser, _ := user.Current()
	paths := strings.Split(EnvCachePaths, ",")

	for i, path := range paths {
		if path == "~" || path == "~/" {
			return fmt.Errorf("EnvCachePaths is using ~/ as a path")
		}

		if strings.HasPrefix(path, "~/") {
			path = filepath.Join(currentUser.HomeDir, path[2:])
			Logger.Debug("converted path %q to %q", paths[i], path)
		}

		if strings.HasSuffix(path, "*") {
			matches, err := filepath.Glob(path)
			if err != nil {
				return err
			}

			for _, m := range matches {
				path = m
				Logger.Debug("converted path %q to %q", paths[i], path)
				break
			}
		}

		if err := cleanupWorkspace(path); err != nil {
			Logger.Warning("unable to remove %s", path)
			return err
		}
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

func makeAbsolutePaths(sourceFileName string, newName string) string {
	srcDir := path.Dir(sourceFileName)

	if len(srcDir) == 0 {
		Logger.Fatal("%s does not appear to be a file", sourceFileName)
	}

	srcFile, err := os.Open(sourceFileName)
	if err != nil {
		Logger.Fatal("%s cannot be read: %v", sourceFileName, err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(filepath.Join(srcDir, newName))
	if err != nil {
		Logger.Fatal("%s cannot be created: %v", newName, err)
	}
	defer destFile.Close()

	infile := bufio.NewScanner(srcFile)
	outfile := bufio.NewWriter(destFile)
	homeDir := os.Getenv("HOME")

	for infile.Scan() {
		ln := infile.Text()

		ln = strings.ReplaceAll(ln, "./", srcDir+"/")
		ln = strings.ReplaceAll(ln, "~/", homeDir+"/")

		if _, err := outfile.WriteString(ln + "\n"); err != nil {
			Logger.Fatal("an unexpected error occurred writing to %s: %v", destFile.Name(), err)
		}
	}

	outfile.Flush()

	if err := infile.Err(); err != nil {
		Logger.Fatal("an unexpected error occurred reading %s: %v", srcFile.Name(), err)
	}

	return destFile.Name()
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

// RunAnsible via ansible
func RunAnsible(ansibleConfig string, args ...string) (bool, int) {
	c := generateCommand(Ansible, args...)
	c.Env = append(os.Environ(), "PEX_SCRIPT=ansible", fmt.Sprintf("ANSIBLE_CONFIG=%s", ansibleConfig))

	Logger.Debug("Executing ansible: %s", c.Env)

	if err := c.Run(); err != nil {
		Logger.Error("failed to execute command '%s': %s", c, err)
		return false, c.ProcessState.ExitCode()
	}

	return true, 0
}

// ShowInventory via ansible-inventory
func ShowInventory(ansibleConfig string, args ...string) (bool, int) {
	c := generateCommand(Ansible, args...)
	c.Env = append(os.Environ(), "PEX_SCRIPT=ansible-inventory", fmt.Sprintf("ANSIBLE_CONFIG=%s", ansibleConfig))

	Logger.Debug("Showing the inventory: %s", c.Env)

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

		// Skip unnecessary files
		if Config.Mode&adhocMode > 0 {
			pthN := strings.SplitN(pth, "/", 5)

			if len(pthN) > 3 && !slices.Contains([]string{"plugins", "default.cfg"}, pthN[3]) {
				Logger.Debug("skipping '%s' due to adhoc Mode", pth)
				continue
			}
		}

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
