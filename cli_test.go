package main

import (
	"os"
	"testing"
)

var envOverrideVars = map[string]string{
	"EDITOR":                        "nano",
	"GASCAN_FLAG_LOG_LEVEL":         "debug",
	"GASCAN_FLAG_PASSWORDLESS_SUDO": "1",
	"GASCAN_FLAG_PLAYBOOK":          "ping.yaml",
	"GASCAN_FLAG_SKIP_TAGS":         "sudo",
	"GASCAN_FLAG_TAGS":              "sudo",
}

func TestConfiguration(t *testing.T) {
	if !checkPlaybook(EntryPointPlaybook) {
		t.Fatalf("EntryPointPlaybook '%s' is absent from the bundle", EntryPointPlaybook)
	}

	// Test env overrides
	for e, v := range envOverrideVars {
		os.Setenv(e, v)
	}

	flags()

	for e, v := range envOverrideVars {
		data := map[string]interface{}{"cfg": "", "exp": ""}

		switch e {
		case "EDITOR":
			data["cfg"] = Config.Editor
			data["exp"] = v
			break
		case "GASCAN_FLAG_LOG_LEVEL":
			data["cfg"] = Config.LogLevel
			data["exp"] = v
			break
		case "GASCAN_FLAG_PASSWORDLESS_SUDO":
			data["cfg"] = Config.NoSudoPassword
			data["exp"] = optInDefaultOff[v]
			break
		case "GASCAN_FLAG_PLAYBOOK":
			data["cfg"] = Config.Playbook
			data["exp"] = v
			break
		case "GASCAN_FLAG_SKIP_TAGS":
			data["cfg"] = Config.SkipTags
			data["exp"] = v
			break
		case "GASCAN_FLAG_TAGS":
			data["cfg"] = Config.Tags
			data["exp"] = v
			break
		}

		if data["cfg"] != data["exp"] {
			t.Fatalf("%s does not match: got %v, expected %v", e, data["cfg"], data["exp"])
		}
	}
}
