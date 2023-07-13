//go:build ignore

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
)

const (
	extractGo = `// Code generated .* DO NOT EDIT\.
package main

const (
	// ExtractAnsibleConfig controls whether the default.cfg should be copied
	// to ~/.ansible.cfg
	ExtractAnsibleConfig = %v

	// ExtractDynamicInventory controls whether the dynamic inventory script
	// should be extracted from the binary
	ExtractDynamicInventory = %v
)
`
	playGo = `// Code generated .* DO NOT EDIT\.
package main

const (
	// PlaybookList is a newline-delimited list of playbooks
	// found in the embedded tarball, avoids inspection
	PlaybookList = "%s"
)
`
	versionGo = `// Code generated .* DO NOT EDIT\.
package main

const (
	// AnsibleVersion for the built-in PEX
	AnsibleVersion = "%s"

	// BundleVersion for the built-in bundle, determined by env BUNDLE_VERSION
	BundleVersion = "%s"

	// PythonVersion for the built-in PEX
	PythonVersion = "%s"

	// Version of the software, determined by env RELEASE_VERSION
	Version = "%s"
)
`
)

var (
	//go:embed bundle.tgz
	bundle []byte

	optInDefaultOn = map[string]bool{"": true, "yes": true, "1": true}
)

func generateVersion() {
	ansibleVersion := strings.ReplaceAll(os.Getenv("ANSIBLE_VERSION"), `"`, "")
	bundleVersion := strings.ReplaceAll(os.Getenv("BUNDLE_RELEASE_VERSION"), `"`, "")
	pythonVersion := strings.ReplaceAll(os.Getenv("PYTHON_VERSION"), `"`, "")
	version := strings.ReplaceAll(os.Getenv("RELEASE_VERSION"), `"`, "")

	if ansibleVersion == "" {
		panic("env ANSIBLE_VERSION is undefined")
	}

	if bundleVersion == "" {
		panic("env BUNDLE_RELEASE_VERSION is undefined")
	}

	if pythonVersion == "" {
		panic("env PYTHON_VERSION is undefined")
	}

	if version == "" {
		panic("env RELEASE_VERSION is undefined")
	}

	if err := ioutil.WriteFile("version.go", []byte(fmt.Sprintf(versionGo, ansibleVersion, bundleVersion, pythonVersion, version)), 0o644); err != nil {
		panic("unable to write version.go")
	}
}

func generateExtract() {
	var extractAnsibleConfig, extractDynamicInventory bool

	if optInDefaultOn[strings.ReplaceAll(os.Getenv("EXTRACT_ANSIBLE_CONFIG"), `"`, "")] {
		extractAnsibleConfig = true
	} else {
		extractAnsibleConfig = false
	}

	if optInDefaultOn[strings.ReplaceAll(os.Getenv("EXTRACT_DYNAMIC_INVENTORY"), `"`, "")] {
		extractDynamicInventory = true
	} else {
		extractDynamicInventory = false
	}

	if err := ioutil.WriteFile("extract.go", []byte(fmt.Sprintf(extractGo, extractAnsibleConfig, extractDynamicInventory)), 0o644); err != nil {
		panic("unable to write extract.go")
	}
}

func generatePlaybook() {
	var plays []string

	buf := bytes.NewBuffer(bundle)
	gbuf, err := gzip.NewReader(buf)
	if err != nil {
		panic("unable to read with gzip")
	}

	tr := tar.NewReader(gbuf)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic("failed to extract tarball")
		}

		// Fail if an unexpected prefix exists, or the path ascends the directory tree
		if strings.Contains(hdr.Name, "..") {
			continue
		}

		pth := strings.Replace(hdr.Name, "automation/", "", 1)
		if !strings.HasSuffix(pth, ".yaml") || strings.Contains(pth, "/") {
			continue
		}
		plays = append(plays, pth)
	}

	sort.Strings(plays)

	if err := ioutil.WriteFile("playbook.go", []byte(fmt.Sprintf(playGo, strings.Join(plays, ","))), 0o644); err != nil {
		panic("unable to write extract.go")
	}
}

func main() {
	genExtractFlag := flag.Bool("generate-extract", false, "Generate extract.go")
	genPlaybookFlag := flag.Bool("generate-playbook", false, "Generate playbook.go")
	genVersionFlag := flag.Bool("generate-version", false, "Generate version.go")

	flag.Parse()

	if *genExtractFlag {
		generateExtract()
	}

	if *genPlaybookFlag {
		generatePlaybook()
	}

	if *genVersionFlag {
		generateVersion()
	}
}
