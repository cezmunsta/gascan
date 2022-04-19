package main

import (
	"flag"
	"strings"
)

// Flags provides configuration options
type Flags struct {
	Editor         string
	EnableGodMode  bool
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
var EntryPointPlaybook = "pmm-server.yaml"

func flags() {
	Config.Mode = 0

	noConfigFlag := flag.Bool("skip-configure", false, "Skip initial configuration")
	noDeployFlag := flag.Bool("skip-deploy", false, "Skip deploying the monitor host")
	testFlag := flag.Bool("test", false, "Run the test play (ping)")

	flag.BoolVar(&Config.NoSudoPassword, "passwordless-sudo", false, "The use of sudo does not require a password")

	flag.StringVar(&Config.Editor, "editor", "vi", "Path to preferred editor")
	flag.StringVar(&Config.Inventory, "inventory", "", "Set a custom inventory")
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
}
