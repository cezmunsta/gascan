package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	configMode  uint = 2
	deployMode  uint = 4
	testMode    uint = 8
	extractMode uint = 16
)

var (
	// Ansible stores the path to the extracted executable
	Ansible string

	// Config stores the settings
	Config Flags

	// DynamicInventoryScript is the path to the extracted Python script
	// to use as a dynamic inventory
	DynamicInventoryScript string

	// Logger handles log output
	Logger Log

	//go:embed bundle.tgz
	bundle []byte

	defaultConfig = "templates/defaultConfig.j2"

	defaultInventory = "templates/defaultInventory.j2"

	//go:embed scripts/dynamic-inventory/get_inventory.py
	dynamicInventory []byte

	isDone bool

	//go:embed build/ansible
	pex []byte
)

// Template ensures that template.Template can be rendered
type Template interface {
	render(tmpl *template.Template) []byte
}

type ansibleInventory struct {
	Config Flags
}

func (a *ansibleInventory) render(tmpl *template.Template) []byte {
	content, err := renderTemplate(a, tmpl)
	if err != nil {
		Logger.Fatal("failed to render template '%s': %v", tmpl.Name(), err)
	}

	return content
}

func renderTemplate(content Template, tmpl *template.Template) ([]byte, error) {
	buf := bytes.Buffer{}

	if err := tmpl.Execute(&buf, content); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func generateCommand(cmd string, args ...string) *exec.Cmd {
	c := exec.Command(cmd, args...)

	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
	c.Stdin = os.Stdin

	return c
}

func main() {
	flags()

	isDone = false
	tmpDir := createWorkspace()

	Ansible = filepath.Join(tmpDir, "ansible.pex")
	DynamicInventoryScript = filepath.Join(tmpDir, "dynamic-inventory.py")

	inventory := Config.Inventory
	playArgs := []string{}
	pp := filepath.Join(tmpDir, Config.Playbook)
	tp := filepath.Join(tmpDir, "ping.yaml")

	if len(Config.Tags) > 0 {
		playArgs = append(playArgs, "--tags", Config.Tags)
	}
	if len(Config.SkipTags) > 0 {
		playArgs = append(playArgs, "--skip-tags", Config.SkipTags)
	}
	if !Config.NoSudoPassword {
		playArgs = append(playArgs, "--ask-become-pass")
	}

	defer func() {
		if isDone {
			cleanupWorkspace(tmpDir)
		} else {
			a := strings.Join(playArgs, " ")
			Logger.Info("Your workspace has been left in place:", tmpDir)
			fmt.Println("Ansible:", Ansible)
			fmt.Println("Run ping test: ANSIBLE_CONFIG="+strings.Replace(Ansible, "ansible.pex", "default.cfg", 1), "PEX_SCRIPT=ansible-playbook ", Ansible, a, tp)
			fmt.Println("Run deploy: ANSIBLE_CONFIG="+strings.Replace(Ansible, "ansible.pex", "default.cfg", 1), "PEX_SCRIPT=ansible-playbook ", Ansible, a, pp)
		}
	}()

	extractBundle(bundle, tmpDir)

	if _, err := os.Stat(inventory); err != nil {
		if inventory != "" {
			Logger.Warning("unable to locate %s, default inventory will be used instead", inventory)
		}
		inventory = filepath.Join(tmpDir, "temp-inventory.yaml")
		generateDefaults(inventory)
	}

	playArgs = append(playArgs, "--inventory", inventory)

	extractToFile(Ansible, pex, 0o550)
	extractToFile(DynamicInventoryScript, dynamicInventory, 0o550)

	if Config.Mode&extractMode > 0 {
		os.Exit(0)
	}

	if Config.Mode&configMode > 0 {
		Logger.Debug("Opening inventory for editing")
		tpls := inventory

		if err := editTemplates(tpls); err != nil {
			Logger.Fatal("unable to make the necessary configuration changes: %v", err)
		}
	}

	if Config.Mode&testMode > 0 {
		a := append([]string{tp}, playArgs...)
		RunPlaybook(a...)
	}

	if Config.Mode&deployMode > 0 {
		a := append([]string{pp}, playArgs...)

		isDone = RunPlaybook(a...)
	}
}
