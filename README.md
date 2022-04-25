# gascan

Go deploy tool to manage Percona Platform with Ansible

==This tool is in proof-of-concept phase==

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

### System requirements

Almost all of the functionality is built in to the binary and so the requirement to use the tool are kept to a minimum. However, depending upon the Linux distribution that you are deploying on it may be necessary to ensure that some requirements are met.

#### Operating systems

The following operating systems are supported:
* CentOS 7
* CentOS Stream 8
* CentOS Stream 9
* Red Hat Enterprise Linux 8
* Debian 11
* Ubuntu 22.04

#### Package updates

As a best practice, ensure that all system packages are up-to-date and the server has been rebooted if the kernel was updated. It may be possible to have success without following this, however depending upon the existing package versions and the options chosen your mileage may vary. At a bare minimum, ensuring that you are running the the latest available kernel for your distribution will help avoid the most common issues.

#### Python versions

The following versions of Python are supported:
* Python 3.8
* Python 3.9

Whilst almost all types of installation of the supported Linux distributions will have Python already available, it may be necessary to either install a specific version or some additional packages.

##### CentOS 7

Install the software compatibility library (SCL) repository to gain access to the Red Hat packages for Python 3.8:
```sh
$ sudo yum install centos-release-scl
$ sudo yum install rh-python38
```

You can then make this available to your session in a number of ways, the easiest being one of the following:
```sh
$ source /opt/rh/rh-python38/enable
# OR
$ sudo update-alternatives --install /usr/bin/python3.8 python3.8 /opt/rh/rh-python38/root/bin/python3.8 100
```

##### CentOS Stream8

Depending upon the installation, it may be necessary to install one of the following packages:
* python3.8
* python3.9

```sh
$ sudo dnf install python3.8
```

##### Debian 11

Depending upon the installation, it may be necessary to install the following package:
* python3-distutils

```sh
$ sudo apt install python3-distutils
```

###### Known issues {#debian-known-issues}
There is a known issue where an error will occur whilst attempting to perform a full execution.
On RHEL-based distributions, `/usr/bin/command` exists to provide access to the shell builtin `command`:
```sh
#!/bin/sh
builtin command "$@"
```

The following provides a workaround on Debian, assuming that `${HOME}/bin` is in your `PATH`:
```sh
$ cat <<EOS > ${HOME}/bin/command
#!/bin/sh
command "\$@"
EOS

$ chmod u+x ~/bin/command
```

##### Ubuntu 22.04

As with Debian 11, a [workaround](#debian-known-issues) is required unless `--skip-configure` is used.

### Examples

#### Test-only mode
```sh
$ gascan --test --skip-configure --skip-deploy --monitor=dummy-monitor
```

#### Configuration-only mode
```sh
$ gascan --skip-deploy --monitor=dummy-monitor
```

#### Run sudo-less tasks
```sh
$ gascan --skip-tags=sudo --monitor=dummy-monitor
```

#### Run with a pre-defined inventory
```sh
$ gascan --skip-configure --inventory /path/to/inventory --monitor=dummy-monitor
```

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

#### Build for Ubuntu 22.04
```
$ PY=3.10 VERSION=jammy BUILD_BASE=ubuntu:jammy make all
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
