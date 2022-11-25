#!/bin/bash -x
# buildah bud -f images/ansible/Containerfile --squash --no-cache --force-rm --compress --tag msp-ansible
# podman run --rm -it -v ./:/app
# podman run --rm -it -v ./:/app 3.9
# PEX_UNZIP=1 PEX_ROOT=/tmp/foo ./ansible3.8 --help
set -eu

declare -r DISTRO="${1}"
declare -r PACKAGES="${2:-undef}"

declare USERNAME="${USER_NAME:-percona}"
declare -i USERUID=${USER_UID:-1000}

function setup {
    install -o "${USERNAME}" -m 0755 -d /app

    case "${DISTRO}" in
        # TODO: add support for UBI - ubi, ubi-minimal, ubi-init
        ## https://developers.redhat.com/products/rhel/ubi
        "centos:stream8"|"centos:stream9") setup_redhat "${@}";;
        "centos:7") setup_redhat_legacy "${@}";;
        "ubuntu:22.04"|"ubuntu:jammy"|"debian:bullseye"|"debian:11") setup_debian "${@}";;
        #"python:3.10-slim") ;;
        *) echo "Unsupported distro: ${DISTRO}"; exit 1
    esac
}

function setup_debian {
    local -r extra_packages="${1}"
    local -a packages

    if [ -f "${extra_packages}" ]; then
        mapfile -t packages < "${extra_packages}"
    fi

    case "${DISTRO}" in
        *) packages+=( python3-minimal python3-venv )
    esac

    apt update -qqy
    apt install -qqy "${packages[@]}"
    apt clean -qqy
}

function setup_redhat {
    local -r extra_packages="${1}"
    local -a packages
    local -a repos

    if [ -f "${extra_packages}" ]; then
        mapfile -t packages < "${extra_packages}"
    fi

    dnf makecache -y

    case "${DISTRO}" in
        "centos:stream8") packages+=( python38 python38-wheel python39 python39-wheel );;
        "centos:stream9") packages+=( python3 python-wheel-wheel ); repos=( "--enablerepo=crb" );
    esac

    dnf install -y "${repos[@]}" "${packages[@]}"
    dnf clean all
}

function setup_redhat_legacy {
    local -r extra_packages="${1}"
    local -a packages

    if [ -f "${extra_packages}" ]; then
        mapfile -t packages < "${extra_packages}"
    fi

    case "${DISTRO}" in
        *) packages+=( rh-python38 rh-python38-python-wheel )
    esac

    yum makecache -y
    yum install -y centos-release-scl
    yum install -y "${packages[@]}"
    yum clean all
    update-alternatives --install /usr/bin/python3.8 python3.8 /opt/rh/rh-python38/root/bin/python3.8 100
}

id -un "${USERUID}" || {
    useradd -d /app \
        -s /sbin/nologin \
        -u "${USERUID}" \
        "${USERNAME}"
}

setup "/opt/$(basename "${PACKAGES}")"
