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

## Build options

The following options apply to all of the builds:
* `ARCH` sets the system archiecture, currently limited to `amd64`
* `OS` sets the operating system, currently limited to `linux`
* `VERSION` sets the tag for container images and builds

### Ansible PEX
* `BUILD_BASE` sets base container image that is used to create the build container
* `PY` sets the version of Python to build for

### Gascan
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

#### Regenerate PEX executables

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


