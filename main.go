package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
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

	// ApiEndpoint defines the target for requesting an inventory
	ApiEndpoint = "http://localhost/inventory"

	// Config stores the settings
	Config Flags

	// DynamicInventoryScript is the path to the extracted Python script
	// to use as a dynamic inventory
	DynamicInventoryScript string

	// HeaderIdentifier sets the header name for the client identifier
	HeaderIdentifier = "Auth-Id"

	// HeaderMonitorName sets the header name for the monitor name
	HeaderMonitorName = "Monitor-Name"

	// HeaderToken sets the header name for the token
	HeaderToken = "Auth-Token"

	// Logger handles log output
	Logger Log

	//go:embed scripts/ansible/bin-helper.sh
	binHelper []byte

	//go:embed bundle.tgz
	bundle []byte

	defaultConfig = "templates/defaultConfig.j2"

	defaultInventory = "templates/defaultInventory.j2"

	//go:embed scripts/dynamic-inventory/get_inventory.py
	dynamicInventory []byte

	isDone bool

	//go:embed build/ansible
	pex []byte

	sampleInventoryConfig SampleInventoryConfig
)

type SampleInventoryConfig struct {
	Headers          map[string]string `json:"headers"`
	KeyFile          string            `json:"key_file"`
	RetryAttempts    uint              `json:"retry_attempts"`
	RetryWaitSeconds uint              `json:"retry_wait_seconds"`
	Uri              string            `json:"uri"`
}

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

func checkInventoryStatus(inventory string, tmpDir string) (string, error) {
	paths := strings.Split(inventory, ",")
	usableInventory := ""

	for i, p := range paths {
		if _, err := os.Stat(p); err != nil {
			Logger.Debug("ignoring inventory path '%s'", p)
			continue
		}

		sfmt := ",%s"
		if i == 0 {
			sfmt = "%s"
		}

		usableInventory += fmt.Sprintf(sfmt, p)
	}

	if usableInventory == "" {
		usableInventory = filepath.Join(tmpDir, "temp-inventory.yaml")
		generateDefaults(usableInventory)
	}

	if usableInventory != inventory {
		return usableInventory, fmt.Errorf("unable to use '%s' as the inventory, using '%s'", inventory, usableInventory)
	}

	return inventory, nil
}

func generateHash(path string) (string, error) {
	var machineId []byte
	newKey := sha256.New()

	if data, err := ioutil.ReadFile(path); err == nil {
		machineId = []byte(fmt.Sprintf("%s-%s", data, time.Now()))
	}

	if len(machineId) == 0 {
		return "", fmt.Errorf("unable to locate %s", path)
	}

	newKey.Write(machineId)

	return hex.EncodeToString(newKey.Sum(nil)), nil
}

func generateVaultKey(path string) error {
	nk, err := generateHash("/etc/machine-id")
	if err != nil {
		Logger.Fatal("unable to generate hash: %v", err)
	}

	if err := ioutil.WriteFile(path, []byte(nk), 0o400); err != nil {
		Logger.Fatal("failed to create vault key '%s': %v", path, err)
	}

	return nil
}

func prepareHost(baseDir string, binDir string, configDir string) error {
	// binDir := filepath.Join(baseDir, "bin")
	// configDir := filepath.Join(baseDir, ".config", "gascan")

	ansibleHelper := filepath.Join(binDir, "ansible.sh")
	dynInventory := filepath.Join(binDir, "dynamic-inventory.py")
	dynInventorySrc := filepath.Join(baseDir, "dynamic-inventory.py")
	dynInventoryConf := filepath.Join(configDir, "inventory-config.json")
	secrets := filepath.Join(configDir, "secrets.yaml")
	tempInventory := filepath.Join(baseDir, "temp-inventory.yaml")
	vaultKey := filepath.Join(configDir, ".vault-key")

	// Create the config directory
	fmt.Println("Creating config directory:", configDir)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		Logger.Fatal("failed to create config directory '%s': %v", configDir, err)
	}

	// Generate a key for Ansible encryption
	if _, err := os.Stat(vaultKey); err != nil {
		fmt.Println("Creating vault key:", vaultKey)
		generateVaultKey(vaultKey)
	}

	// Create the bin directory
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		Logger.Fatal("failed to create bin directory '%s': %v", binDir, err)
	}

	// Create the Ansible PEX helper and symlinks
	if _, err := os.Stat(ansibleHelper); err != nil {
		symlinks := []string{"ansible", "ansible-playbook", "ansible-vault", "ansible-config", "ansible-inventory"}

		fmt.Println("Creating Ansible helper script:", ansibleHelper)
		extractToFile(ansibleHelper, binHelper, 0o750)

		for _, p := range symlinks {
			os.Symlink(ansibleHelper, filepath.Join(binDir, p))
		}
	}

	// Copy the generated inventory to use for secrets
	if _, err := os.Stat(secrets); err != nil {
		if _, err := os.Stat(tempInventory); err != nil {
			generateDefaults(tempInventory)
		}

		if c, err := ioutil.ReadFile(tempInventory); err == nil {
			fmt.Printf("Copying temporary inventory '%s' to '%s'\n", tempInventory, secrets)
			extractToFile(secrets, c, 0o600)
		}
	}

	// Copy the dynamic inventory script
	if c, err := ioutil.ReadFile(dynInventorySrc); err == nil {
		fmt.Printf("Copying dynamic inventory '%s' to '%s'\n", dynInventorySrc, dynInventory)
		extractToFile(dynInventory, c, 0o550)
	}

	// Generate a config for the dynamic inventory
	if _, err := os.Stat(dynInventoryConf); err != nil {
		hi, _ := generateHash("/etc/machine-id")
		ht, _ := generateHash("/etc/machine-id")

		sampleInventoryConfig = SampleInventoryConfig{
			Headers: map[string]string{
				"Content-type":    "application/json",
				HeaderIdentifier:  hi,
				HeaderMonitorName: Config.Monitor,
				HeaderToken:       ht,
			},
			KeyFile:          vaultKey,
			RetryAttempts:    3,
			RetryWaitSeconds: 10,
			Uri:              ApiEndpoint,
		}

		buf, err := json.MarshalIndent(sampleInventoryConfig, "", "  ")
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(dynInventoryConf, []byte(string(buf)), 0o640); err != nil {
			return err
		}
	}

	return nil
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
	extractToFile(Ansible, pex, 0o550)
	extractToFile(DynamicInventoryScript, dynamicInventory, 0o550)

	if newPath, err := checkInventoryStatus(inventory, tmpDir); err != nil {
		Logger.Warning("unable to locate inventory '%s', '%s' will be used instead", inventory, newPath)
		inventory = newPath
	}

	if os.Getenv("ANSIBLE_INVENTORY") == "" || !strings.Contains(inventory, ",") {
		playArgs = append(playArgs, "--inventory", inventory)
	}

	if Config.Mode&extractMode > 0 {
		bd := filepath.Join(os.Getenv("HOME"), "bin")
		cd := filepath.Join(os.Getenv("HOME"), ".config", "gascan")
		fmt.Println("Extracting bundle to:", tmpDir)
		prepareHost(tmpDir, bd, cd)
		fmt.Println("Helpers created in:", bd)
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
