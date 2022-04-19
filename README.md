# gascan

Go deploy tool to manage Percona Platform with Ansible

## Usage

```sh
Usage of ./build/linux/amd64/gascan:
  -editor string
        Path to preferred editor (default "vi")
  -inventory string
        Set a custom inventory
  -log-level uint
        Set the level of logging verbosity (default 30)
  -monitor string
        Monitor alias (default "monitor")
  -passwordless-sudo
        The use of sudo does not require a password
  -playbook string
        Playbook used for deployment (default "pmm-server.yaml")
  -skip-configure
        Skip initial configuration
  -skip-deploy
        Skip deploying the monitor host
  -skip-tags string
        Specify tags to skip for automation
  -tags string
        Specify tags for automation
  -test
        Run the test play (ping)
```

### Examples


## Build options

The following options apply to all of the builds:
* `ARCH` sets the system archiecture, currently limited to `amd64`
* `BUNDLE` when set will use a custom tarball instead of generating one
* `OS` sets the operating system, currently limited to `linux`
* `VERSION` sets the tag for container images and builds

Executable files are generated to "${BUILD_DIR}/${OS}/${ARCH}", e.g:
```sh
$ ls -1 build/linux/amd64
ansible3.8
gascan-py3.8
```

For ease, the latest generated `gascan` binary can be found at "${BUILD_DIR}/gascan".

### Ansible PEX
* `BUILD_BASE` sets base container image that is used to create the build container
* `PY` sets the version of Python to build for

### Gascan
* `ENTRYPOINT` sets a custom playbook as the default for `-playbook`
* `GO` sets the Golang version
* `GOFMT` sets the formatting tool
* `GOLINT` sets the linter

### Examples

#### Standard build
```sh
# Build everything
$ make all

# Build separately
$ make ansible
$ make build
```

#### Build for Python 3.9
```sh
$ PY=3.9 make all
```

#### Regenerate executables

When the container image is already available for use, it is possible
to shorten the build time to generate the PEX versions.
```sh
$ make ansible_pex
$ PY=3.9 make ansible_pex
```

The binary can then be generated as follows:
```sh
$ PY=3.9 make build
```

#### Use a custom bundle and entrypoint

Instead of using the built-in automation, it may be desirable to use
either a custom bundle, entrypoint, or both.

Generate a sample bundle as a starting point for the custom automation:
```sh
$ make sample-bundle
$ tar -tvf sample-bundle.tgz
```

Once you have made changes to the bundle then you can generate the binary
as follows:
```sh
$ BUNDLE=my-custom-bundle.tgz make build
```

If you wish to also change the default entrypoint (`-playbook`) then
override the entrypoint as well as the bundle:
```sh
$ BUNDLE=my-custom-bundle.tgz ENTRYPOINT=my-custom-playbook.yaml make build
```

The entrypoint is relative to the automation directory, such that the
default entrypoint for the automation is `automation/pmm-server.yaml` and
would be defined as `pmm-server.yaml`
