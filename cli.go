package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
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

func checkPlaybook(play string) {
	exists := false
	for _, p := range strings.Split(PlaybookList, ",") {
		if play == p {
			exists = true
			break
		}
	}

	if !exists {
		Logger.Fatal("Playbook %s is unavailable, please use --list-plays to see what's available", play)
	}
}

func printVersion() {
	fmt.Println("Version:", Version)
	fmt.Println("Go Version:", runtime.Version())
	fmt.Println("Arch:", runtime.GOOS, runtime.GOARCH)
	fmt.Println("Ansible Version:", AnsibleVersion)
	fmt.Println("Bundle Version:", BundleVersion)
	fmt.Println("Python Version:", PythonVersion)
}

func flags() {
	Config.Mode = 0

	envInventory := os.Getenv("ANSIBLE_INVENTORY")
	envBecomePass := os.Getenv("ANSIBLE_BECOME_PASSWORD")
	envBecomePassFile := os.Getenv("ANSIBLE_BECOME_PASSWORD_FILE")

	needsBecomePass := true
	if envBecomePass != "" || envBecomePassFile != "" {
		needsBecomePass = false
	}

	extractOnlyFlag := flag.Bool("extract-bundle", false, "Just extract the bundle, use with --extract-path")
	generateHashFlag := flag.Bool("generate-hash", false, "Generate a sha256 time-based hash")
	listPlaysFlag := flag.Bool("list-plays", false, "List the available playbooks")
	noConfigFlag := flag.Bool("skip-configure", false, "Skip initial configuration")
	noDeployFlag := flag.Bool("skip-deploy", false, "Skip deploying the monitor host")
	testFlag := flag.Bool("test", false, "Run the test play (ping)")
	versionFlag := flag.Bool("version", false, "Show the version")

	flag.BoolVar(&Config.NoSudoPassword, "passwordless-sudo", !needsBecomePass, "The use of sudo does not require a password")

	flag.StringVar(&Config.Editor, "editor", "vi", "Path to preferred editor")
	flag.StringVar(&Config.ExtractPath, "extract-path", os.TempDir(), "Extract the bundle to this path, use with --extract-bundle, when TMPDIR cannot execute, etc")
	flag.StringVar(&Config.Inventory, "inventory", envInventory, "Set a custom inventory")
	flag.StringVar(&Config.LogLevel, "log-level", "error", "Set the level of logging verbosity")
	flag.StringVar(&Config.Monitor, "monitor", "monitor", "Monitor alias")
	flag.StringVar(&Config.Playbook, "playbook", EntryPointPlaybook, "Playbook used for deployment")
	flag.StringVar(&Config.SkipTags, "skip-tags", "", "Specify tags to skip for automation")
	flag.StringVar(&Config.Tags, "tags", "", "Specify tags for automation")

	flag.Parse()

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

	if *versionFlag {
		printVersion()
		os.Exit(0)
	}

	if *listPlaysFlag {
		fmt.Println(strings.ReplaceAll(PlaybookList, ",", "\n"))
		os.Exit(0)
	}

	checkPlaybook(Config.Playbook)

	if *generateHashFlag {
		hash, err := generateHash("/etc/machine-id")
		if err != nil {
			panic(err)
		}

		fmt.Println(hash)
		os.Exit(0)
	}

	if *testFlag {
		Config.Mode += testMode
	}
	if !*noConfigFlag && Config.Inventory == "" && optInDefaultOn[os.Getenv("GASCAN_DEFAULT_INVENTORY")] {
		Config.Mode += configMode
	}
	if !*noDeployFlag {
		Config.Mode += deployMode
	}
	if *extractOnlyFlag {
		Config.Mode = extractMode
	}
}
