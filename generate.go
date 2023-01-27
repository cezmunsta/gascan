//go:build ignore

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

const versionGo = `// Code generated .* DO NOT EDIT\.
package main

const (
    // AnsibleVersion for the built-in PEX
    AnsibleVersion = "%s"

    // PythonVersion for the built-in PEX
    PythonVersion = "%s"

    // Version of the software, determined by env RELEASE_VERSION
    Version = "%s"
)
`

func main() {
	ansible_version := strings.ReplaceAll(os.Getenv("ANSIBLE_VERSION"), `"`, "")
	python_version := strings.ReplaceAll(os.Getenv("PYTHON_VERSION"), `"`, "")
	version := strings.ReplaceAll(os.Getenv("RELEASE_VERSION"), `"`, "")

	if ansible_version == "" {
		panic("env ANSIBLE_VERSION is undefined")
	}

	if python_version == "" {
		panic("env PYTHON_VERSION is undefined")
	}

	if version == "" {
		panic("env RELEASE_VERSION is undefined")
	}

	if err := ioutil.WriteFile("version.go", []byte(fmt.Sprintf(versionGo, ansible_version, python_version, version)), 0o644); err != nil {
		panic("unable to write version.go")
	}
}
