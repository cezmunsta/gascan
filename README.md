[![Dependencies](https://github.com/cezmunsta/gascan/actions/workflows/dependency-review.yml/badge.svg)](https://github.com/cezmunsta/gascan/actions/workflows/dependency-review.yml)
[![CodeQL](https://github.com/cezmunsta/gascan/actions/workflows/codeql.yml/badge.svg)](https://github.com/cezmunsta/gascan/actions/workflows/codeql.yml)

# gascan

Go deploy tool to manage Percona Platform with Ansible

==This tool is in proof-of-concept phase==

## Usage

```sh
Usage of gascan:
  -editor string
        Path to preferred editor (default "vi")
  -extract-bundle
        Just extract the bundle, use with --extract-path
  -extract-path string
        Extract the bundle to this path, use with --extract-bundle, when TMPDIR cannot execute, etc (default "/tmp")
  -generate-hash
        Generate a sha256 time-based hash
  -inventory string
        Set a custom inventory
  -log-level string
        Set the level of logging verbosity (default "error")
  -monitor string
        Monitor alias (default "monitor")
  -passwordless-sudo
        The use of sudo does not require a password
  -playbook string
        Playbook used for deployment (default "pmm-full.yaml")
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
* Red Hat Enterprise Linux 9
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

#### Extract the bundle
```sh
# Using the default extract directory
$ gascan --extract-bundle
Extracted bundle to: /tmp/onboarding1369301009

# Using a specific extract directory
$ gascan --extract-bundle --extract-path="${HOME}/tmp"
Extracted bundle to: /home/user/tmp/onboarding1369301009
```

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
# Specify the inventory via the --inventory flag
$ gascan --inventory /path/to/inventory --monitor=dummy-monitor

# Specify the inventory via ANSIBLE_INVENTORY environment variable
$ export ANSIBLE_INVENTORY=/tmp/foo.yaml
$ gascan --monitor=dummy-monitor
```

## Design decisions for gascan

### CLI usage

- No need to have Ansible already available
  PEX is bundled in the binary, along with the tarball containing the automation code for Ansible to use,
  only requiring an equivalent Python version and the binary. As bundling the PEX increases the binary size, an option
  to create a virtual environment directly is planned, to allow for a slimline binary to be generated as well.
- Use built-in automation, or user's own
  The binary can be built with a custom bundle instead of the default bundle, which means that a user can make adjustments
  for their own infrastructure and repackage it. In addition, the user can choose a different playbook to execute, so
  long as the playbook resides in the bundle.
- Use a generated inventory, or user's own
  During a full run, a inventory is generated and the user is given the opportunity to edit it if required. It is also
  possible to skip this and use a custom inventory, or reuse an existing one
- Run without the need for sudo, tasks using `become` use the `sudo` tag
  To allow for environments where administrative privileges may not be available to the user using the tool on a
  day-to-day basis, tasks that require sudo can run independently and the request for a password can be disabled.
- Enable an administrator to enter the password when needing to escalate privileges
  Whilst the user can either set `ansible_become_pass`, `ANSIBLE_BECOME_ASK_PASS`, `become_password_file`, or
  `ANSIBLE_BECOME_PASSWORD_FILE` to avoid the need to enter the password, it may be that an administrator needs to
  enter the password for the user, or run the administrative tasks ahead of time.
- Failed runs allow for the user to continue with the extracted bundle
  When a run fails, the extracted bundle is left in place, allowing for inplace fixes, or adjustments to be made

### Deployment

- Easily get the basics up and running (server and agents)
  The main aim of the tool is to ease and standardise installations, so the user is left to use PMM directly for
  certain tasks, although over time more automation will be added.
- Run PMM Server with `podman` as a `systemd` service with an unprivileged user
  For a more secure installation, the containers run as an unprivileged user and also are destroyed when the
  service stops. This reduces the scope for meddling with the container, whether that be benign or otherwise,
  and discourages editing of files etc.
- Run PMM Agent without the need for administrative privileges
  By default, the agent is installed via tarball and configured as a service in user's service manager.
  Linger is set to allow it to run when the user is not logged in, although it requires starting after a reboot.
  An administrator could replicate the service to be a system service if required.
- Prefer use of the API whenever possible
  Use of Grafana's and PMM's APIs are preferred over executing commands
- Change the admin default password
  Request a new password when the default admin password is detected and update the inventory.
- Enable use of API tokens instead of user accounts

## Build options

The following options apply to all of the builds:
* `ARCH` sets the system archiecture, currently limited to `amd64`
* `BUILD_DIR` sets the base output directory for the Go binaries
* `BUNDLE` when set will use a custom tarball instead of generating one
* `OS` sets the operating system, currently limited to `linux`
* `VERSION` sets the tag for container images and builds

Executable files are generated to "${BUILD_DIR}/${OS}/${ARCH}/${BUILD_BASE_TAG}", e.g:
```sh
$ ls -1 build/linux/amd64/centos-stream8
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

There are some additional build variables that have an effect on the generation of the
sample config for the dynamic inventory script, which is generated via the `--extract-bundle`
option:
* `AUTH_FIELD_1` sets the first header used, default `Auth-Id`
* `AUTH_FIELD_2` sets the second header used, default `Auth-Token`
* `AUTH_FIELD_3` sets the final header used, default `Monitor-Name`

### Examples

#### Standard build
```sh
# Build everything for the default version
$ make all

# Build separately
$ make ansible
$ make build
```

#### Build all versions
```sh
$ make all_versions | xargs -L1 make
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
