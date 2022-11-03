package main

import (
	"flag"
	"os"
	"strings"
)

// Flags provides configuration options
type Flags struct {
	Editor         string
	EnableGodMode  bool
	ExtractPath    string
	Inventory      string
	LogLevel       string
	Mode           uint
	Monitor        string
	NoSudoPassword bool
	Playbook       string
	SkipTags       string
	Tags           string
	Test           bool
}

// EntryPointPlaybook defines the playbook that is executed at runtime
var EntryPointPlaybook = "pmm-full.yaml"

func flags() {
	Config.Mode = 0

	envInventory := os.Getenv("ANSIBLE_INVENTORY")
	envBecomePass := os.Getenv("ANSIBLE_BECOME_PASSWORD")
	envBecomePassFile := os.Getenv("ANSIBLE_BECOME_PASSWORD_FILE")

	needsBecomePass := true
	if envBecomePass != "" || envBecomePassFile != "" {
		needsBecomePass = false
	}

	defaultExtractPath, err := os.Getwd()
	if err != nil {
		defaultExtractPath = os.TempDir()
	}

	extractOnlyFlag := flag.Bool("extract-bundle", false, "Just extract the bundle, use with --extract-path")
	noConfigFlag := flag.Bool("skip-configure", false, "Skip initial configuration")
	noDeployFlag := flag.Bool("skip-deploy", false, "Skip deploying the monitor host")
	testFlag := flag.Bool("test", false, "Run the test play (ping)")

	flag.BoolVar(&Config.NoSudoPassword, "passwordless-sudo", needsBecomePass == false, "The use of sudo does not require a password")

	flag.StringVar(&Config.Editor, "editor", "vi", "Path to preferred editor")
	flag.StringVar(&Config.ExtractPath, "extract-path", defaultExtractPath, "Extract the bundle to this path, use with --extract-bundle")
	flag.StringVar(&Config.Inventory, "inventory", envInventory, "Set a custom inventory")
	flag.StringVar(&Config.LogLevel, "log-level", "error", "Set the level of logging verbosity")
	flag.StringVar(&Config.Monitor, "monitor", "monitor", "Monitor alias")
	flag.StringVar(&Config.Playbook, "playbook", EntryPointPlaybook, "Playbook used for deployment")
	flag.StringVar(&Config.SkipTags, "skip-tags", "", "Specify tags to skip for automation")
	flag.StringVar(&Config.Tags, "tags", "", "Specify tags for automation")

	switch strings.ToLower(Config.LogLevel) {
	case "debug":
		Logger.Level = debugLevel
		Logger.Prefix = "DEBUG"
	case "info":
		Logger.Level = infoLevel
		Logger.Prefix = "INFO"
	case "warning", "warn":
		Logger.Level = warningLevel
		Logger.Prefix = "WARNING"
	case "fatal":
		Logger.Level = fatalLevel
		Logger.Prefix = "FATAL"
	default:
		Logger.Level = errorLevel
		Logger.Prefix = "ERROR"
	}

	flag.Parse()

	if *testFlag {
		Config.Mode += testMode
	}
	if !*noConfigFlag {
		Config.Mode += configMode
	}
	if !*noDeployFlag {
		Config.Mode += deployMode
	}
	if *extractOnlyFlag {
		Config.Mode = extractMode
	}
}
