package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

const (
	configMode    uint = 2
	deployMode    uint = 4
	testMode      uint = 8
	extractMode   uint = 16
	inventoryMode uint = 32
	adhocMode     uint = 64

	extractMessage string = `
# Add the following to your shell profile:
export ANSIBLE_VAULT_PASSWORD_FILE='%s' \
       GASCAN_DEFAULT_INVENTORY=0 \
       GASCAN_INVENTORY_CONFIG_FILE='%s'

# Next steps:
## If you need to set your API credentials in your CMDB
Identifier: %s
Token: %s

## If you need to install PMM
gascan --monitor=%s --playbook=pmm-server.yaml%s

## To protect any secrets
ANSIBLE_VAULT_PASSWORD_FILE=%s ansible-vault encrypt %s

`
)

var (
	// Ansible stores the path to the extracted executable
	Ansible string

	// APIEndpoint defines the target for requesting an inventory
	APIEndpoint = "http://localhost/inventory"

	// Config stores the settings
	Config Flags

	// ConnectionTool is the path to the SSH and database connection tool
	ConnectionTool string

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

	//go:embed scripts/connect/connect.py
	connectTool []byte

	defaultConfig = "templates/defaultConfig.j2"

	defaultInventory = "templates/defaultInventory.j2"

	//go:embed scripts/dynamic-inventory/get_inventory.py
	dynamicInventory []byte

	exitCode int

	isDone bool

	optInDefaultOn  = map[string]bool{"": true, "true": true, "yes": true, "1": true}
	optInDefaultOff = map[string]bool{"true": true, "yes": true, "1": true}

	//go:embed build/ansible
	pex []byte

	sampleInventoryConfig SampleInventoryConfig
)

// SampleConnectionToolConfig helps to generate a sample config for dynamic inventories
type SampleConnectionToolConfig struct {
	ServerAddress string            `json:"server_address"`
	Standardise   bool              `json:"standardise"`
	Inventory     map[string]string `json:"inventory"`
}

// SampleInventoryConfig helps to generate a sample config for dynamic inventories
type SampleInventoryConfig struct {
	Headers          map[string]string `json:"headers"`
	KeyFile          string            `json:"key_file"`
	RetryAttempts    uint              `json:"retry_attempts"`
	RetryWaitSeconds uint              `json:"retry_wait_seconds"`
	URI              string            `json:"uri"`
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
	var machineID []byte
	newKey := sha256.New()

	if data, err := os.ReadFile(path); err == nil {
		machineID = []byte(fmt.Sprintf("%s-%s", data, time.Now()))
	}

	if len(machineID) == 0 {
		return "", fmt.Errorf("unable to locate %s", path)
	}

	newKey.Write(machineID)

	return hex.EncodeToString(newKey.Sum(nil)), nil
}

func generateVaultKey(path string) error {
	nk, err := generateHash("/etc/machine-id")
	if err != nil {
		Logger.Fatal("unable to generate hash: %v", err)
	}

	if err := os.WriteFile(path, []byte(nk), 0o400); err != nil {
		Logger.Fatal("failed to create vault key '%s': %v", path, err)
	}

	return nil
}

func prepareHost(baseDir string, binDir string, configDir string) error {
	ansibleConfig := filepath.Join(os.Getenv("HOME"), ".ansible.cfg")
	ansibleConfigSrc := filepath.Join(baseDir, "default.cfg")
	ansibleHelper := filepath.Join(binDir, "ansible.sh")
	ansiblePex := filepath.Join(binDir, "ansible.pex")
	connectionTool := filepath.Join(binDir, "connect.py")
	connectionToolSrc := filepath.Join(baseDir, "connect.py")
	connectionToolConf := filepath.Join(configDir, "connect-py.json")
	dynInventory := filepath.Join(binDir, "dynamic-inventory.py")
	dynInventorySrc := filepath.Join(baseDir, "dynamic-inventory.py")
	dynInventoryConf := filepath.Join(configDir, "inventory-config.json")
	secrets := filepath.Join(configDir, "secrets.yaml")
	tempInventory := filepath.Join(baseDir, "temp-inventory.yaml")
	vaultKey := filepath.Join(configDir, ".vault-key")

	// Extract default.cfg to ~/.ansible.cfg
	if ExtractAnsibleConfig {
		fmt.Println("Extracting default.cfg to ~/.ansible.cfg")
		if c, err := os.ReadFile(ansibleConfigSrc); err == nil {
			extractToFile(ansibleConfig, c, 0o640)
		}
	}

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

		fmt.Println("Creating Ansible PEX:", ansiblePex)
		extractToFile(ansiblePex, pex, 0o750)

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

		if c, err := os.ReadFile(tempInventory); err == nil {
			fmt.Printf("Copying temporary inventory '%s' to '%s'\n", tempInventory, secrets)
			extractToFile(secrets, c, 0o600)
		}
	}

	// Copy the connection tool
	if c, err := os.ReadFile(connectionToolSrc); err == nil {
		fmt.Printf("Copying connection tool '%s' to '%s'\n", connectionToolSrc, connectionTool)
		extractToFile(connectionTool, c, 0o550)

		for _, p := range []string{"db_connect", "ssh_connect"} {
			os.Symlink(connectionTool, filepath.Join(binDir, p))
		}

	}

	// Generate a config for the connection tool
	if _, err := os.Stat(connectionToolConf); err != nil {
		sampleConnectionToolConfig := SampleConnectionToolConfig{
			ServerAddress: "https://localhost:8443",
			Standardise:   true,
			Inventory:     map[string]string{},
		}

		buf, err := json.MarshalIndent(sampleConnectionToolConfig, "", "  ")
		if err != nil {
			return err
		}

		if err := os.WriteFile(connectionToolConf, []byte(string(buf)), 0o640); err != nil {
			return err
		}
	}

	// Copy the dynamic inventory script
	if ExtractDynamicInventory {
		if c, err := os.ReadFile(dynInventorySrc); err == nil {
			fmt.Printf("Copying dynamic inventory '%s' to '%s'\n", dynInventorySrc, dynInventory)
			extractToFile(dynInventory, c, 0o550)
		}
	}

	hi, _ := generateHash("/etc/machine-id")
	ht, _ := generateHash("/etc/machine-id")

	// Generate a config for the dynamic inventory
	if _, err := os.Stat(dynInventoryConf); err != nil {
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
			URI:              APIEndpoint,
		}

		buf, err := json.MarshalIndent(sampleInventoryConfig, "", "  ")
		if err != nil {
			return err
		}

		if err := os.WriteFile(dynInventoryConf, []byte(string(buf)), 0o640); err != nil {
			return err
		}
	} else {
		var st SampleInventoryConfig
		conf, _ := os.ReadFile(dynInventoryConf)

		if err := json.Unmarshal(conf, &st); err != nil {
			Logger.Warning("unable to parse the inventory config '%s'", dynInventoryConf)
		}

		if val, ok := st.Headers[HeaderIdentifier]; ok {
			hi = val
		}

		if val, ok := st.Headers[HeaderToken]; ok {
			ht = val
		}
	}

	p := ""
	if Config.NoSudoPassword {
		p = " --passwordless-sudo"
	}

	fmt.Printf(extractMessage, vaultKey, dynInventoryConf, hi, ht, Config.Monitor, p, vaultKey, secrets)

	return nil
}

func main() {
	flags()

	exitCode = 0
	isDone = false
	tmpDir := createWorkspace()

	Ansible = filepath.Join(tmpDir, "ansible.pex")
	ConnectionTool = filepath.Join(tmpDir, "connect.py")
	DynamicInventoryScript = filepath.Join(tmpDir, "dynamic-inventory.py")

	ansibleConfig := strings.Replace(Ansible, "ansible.pex", "default.cfg", 1)
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

	if len(Config.LimitHosts) > 0 {
		Logger.Debug("Limiting run to %s", Config.LimitHosts)
		playArgs = append(playArgs, "--limit", Config.LimitHosts)
	}

	defer func() {
		if Config.Mode&adhocMode > 0 {
			isDone = true
			exitCode = 0
		}

		if isDone {
			cleanupWorkspace(tmpDir)
		} else {
			a := strings.Join(playArgs, " ")
			Logger.Info("Your workspace has been left in place:", tmpDir)
			fmt.Println("Ansible:", Ansible)
			fmt.Println("Run ping test: ANSIBLE_CONFIG="+ansibleConfig, "PEX_SCRIPT=ansible-playbook", Ansible, a, tp)
			fmt.Println("Run deploy: ANSIBLE_CONFIG="+ansibleConfig, "PEX_SCRIPT=ansible-playbook", Ansible, a, pp)
		}

		os.Exit(exitCode)
	}()

	extractToFile(Ansible, pex, 0o550)
	extractBundle(bundle, tmpDir)

	if Config.Mode&adhocMode == 0 {
		extractToFile(ConnectionTool, connectTool, 0o550)
		extractToFile(DynamicInventoryScript, dynamicInventory, 0o550)
	}

	if optInDefaultOn[os.Getenv("GASCAN_DEFAULT_INVENTORY")] {
		if newPath, err := checkInventoryStatus(inventory, tmpDir); err != nil {
			Logger.Warning("unable to locate inventory '%s', '%s' will be used instead", inventory, newPath)
			inventory = newPath
		}

		playArgs = append(playArgs, "--inventory", inventory)
	}

	if Config.Mode&adhocMode > 0 {
		a := append(playArgs, Config.ExtraArguments...)
		isDone, exitCode = RunAnsible(ansibleConfig, a...)
	}

	if Config.Mode&extractMode > 0 {
		bd := filepath.Join(os.Getenv("HOME"), "bin")
		cd := filepath.Join(os.Getenv("HOME"), ".config", "gascan")
		fmt.Println("Extracting bundle to:", tmpDir)
		fmt.Println("Helpers created in:", bd)
		prepareHost(tmpDir, bd, cd)
		os.Exit(0)
	}

	if Config.Mode&configMode > 0 {
		Logger.Debug("Opening inventory for editing")
		tpls := inventory

		if err := editTemplates(tpls); err != nil {
			Logger.Fatal("unable to make the necessary configuration changes: %v", err)
		}
	}

	if Config.ClearCache {
		Logger.Debug("clearing the inventory cache")

		if err := clearInventoryCache(); err != nil {
			Logger.Fatal("unable to reset the cache: %v", err)
		}
	}

	if Config.Mode&inventoryMode > 0 {
		Logger.Debug("Requesting the inventory")

		ShowInventory(ansibleConfig, []string{"--list"}...)
		os.Exit(0)
	}

	if Config.Mode&testMode > 0 {
		a := append([]string{tp}, playArgs...)
		RunPlaybook(ansibleConfig, a...)
	}

	if Config.Mode&deployMode > 0 {
		a := append([]string{pp}, playArgs...)
		isDone, exitCode = RunPlaybook(ansibleConfig, a...)
	}
}
