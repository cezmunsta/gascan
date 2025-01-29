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
	ClearCache     bool
	Editor         string
	EnableGodMode  bool
	ExtraArguments []string
	ExtractPath    string
	GetInventory   bool
	Inventory      string
	LimitHosts     string
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

func checkPlaybook(play string) bool {
	exists := false
	for _, p := range strings.Split(PlaybookList, ",") {
		if play == p {
			exists = true
			break
		}
	}

	return exists
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
	envBecomePass := os.Getenv("ANSIBLE_BECOME_PASS")
	envBecomePassFile := os.Getenv("ANSIBLE_BECOME_PASSWORD_FILE")

	envEditor := os.Getenv("EDITOR")
	envLogLevel := os.Getenv("GASCAN_FLAG_LOG_LEVEL")
	envPasswordlessSudo := os.Getenv("GASCAN_FLAG_PASSWORDLESS_SUDO")
	envPlaybook := os.Getenv("GASCAN_FLAG_PLAYBOOK")
	envSkipTags := os.Getenv("GASCAN_FLAG_SKIP_TAGS")
	envTags := os.Getenv("GASCAN_FLAG_TAGS")

	// Set default values for flags using optional environment settings
	needsBecomePass := true
	if envBecomePass != "" || envBecomePassFile != "" || optInDefaultOff[envPasswordlessSudo] {
		needsBecomePass = false
	}

	defaultEditor := "vi"
	if envEditor != "" {
		defaultEditor = envEditor
	}

	defaultLogLevel := "error"
	if envLogLevel != "" {
		defaultLogLevel = envLogLevel
	}

	if PlaybookList == envPlaybook || strings.HasPrefix(PlaybookList, envPlaybook+",") || strings.Contains(PlaybookList, ","+envPlaybook+",") || strings.HasSuffix(PlaybookList, ","+envPlaybook) {
		EntryPointPlaybook = envPlaybook
	}

	adhocModeFlag := flag.Bool("adhoc", false, "Using Ansible in adhoc mode")
	extractOnlyFlag := flag.Bool("extract-bundle", false, "Just extract the bundle, use with --extract-path")
	generateHashFlag := flag.Bool("generate-hash", false, "Generate a sha256 time-based hash")
	listPlaysFlag := flag.Bool("list-plays", false, "List the available playbooks")
	noConfigFlag := flag.Bool("skip-configure", false, "Skip initial configuration")
	noDeployFlag := flag.Bool("skip-deploy", false, "Skip deploying the monitor host")
	testFlag := flag.Bool("test", false, "Run the test play (ping)")
	versionFlag := flag.Bool("version", false, "Show the version")

	flag.BoolVar(&Config.ClearCache, "refresh", false, "Clear inventory caches to allow for a refresh")
	flag.BoolVar(&Config.GetInventory, "get-inventory", false, "Request the Ansible inventory")
	flag.BoolVar(&Config.NoSudoPassword, "passwordless-sudo", !needsBecomePass, "The use of sudo does not require a password [GASCAN_FLAG_PASSWORDLESS_SUDO]")

	flag.StringVar(&Config.Editor, "editor", defaultEditor, "Path to preferred editor [EDITOR]")
	flag.StringVar(&Config.ExtractPath, "extract-path", os.TempDir(), "Extract the bundle to this path, use with --extract-bundle, when TMPDIR cannot execute, etc")
	flag.StringVar(&Config.Inventory, "inventory", envInventory, "Set a custom inventory [ANSIBLE_INVENTORY]. A default inventory is used when empty, which can be disabled [GASCAN_DEFAULT_INVENTORY]")
	flag.StringVar(&Config.LimitHosts, "limit", "", "Limit execution to the specified hosts")
	flag.StringVar(&Config.LogLevel, "log-level", defaultLogLevel, "Set the level of logging verbosity [GASCAN_FLAG_LOG_LEVEL]")
	flag.StringVar(&Config.Monitor, "monitor", "monitor", "Monitor alias")
	flag.StringVar(&Config.Playbook, "playbook", EntryPointPlaybook, "Playbook used for deployment [GASCAN_FLAG_PLAYBOOK]")
	flag.StringVar(&Config.SkipTags, "skip-tags", envSkipTags, "Specify tags to skip for automation [GASCAN_FLAG_SKIP_TAGS]")
	flag.StringVar(&Config.Tags, "tags", envTags, "Specify tags for automation [GASCAN_FLAG_TAGS]")

	flag.Parse()

	Config.ExtraArguments = flag.Args()

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

	if !*adhocModeFlag && !checkPlaybook(Config.Playbook) {
		Logger.Fatal("Playbook %s is unavailable, please use --list-plays to see what's available", Config.Playbook)
	}

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

	if Config.GetInventory {
		Config.Mode = inventoryMode
	}

	if *extractOnlyFlag {
		Config.Mode = extractMode
		os.Setenv("GASCAN_TEST_NOEXIT", "1")
	}

	if *adhocModeFlag {
		if len(Config.ExtraArguments) == 0 {
			Logger.Fatal("please specify extra arguments after -- for adhoc mode")
		}

		Config.Mode = adhocMode
		os.Setenv("GASCAN_TEST_NOEXIT", "1")
	}
}
