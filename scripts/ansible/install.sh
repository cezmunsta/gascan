#!/bin/bash -x
# buildah bud -f images/ansible/Containerfile --squash --no-cache --force-rm --compress --tag msp-ansible
# podman run --rm -it -v ./:/app
# podman run --rm -it -v ./:/app 3.9
# PEX_UNZIP=1 PEX_ROOT=/tmp/foo ./ansible3.8 --help
set -eu

declare -r VERSION="${1:-3.9}"
declare -r ANSIBLE="${2:-6.7.0}"
declare -r PACKAGES="${3:-undef}"
declare -r REQUIREMENTS='/tmp/requirements.txt'

function build_pex {
    /app/venv/bin/pex -r "${REQUIREMENTS}" -c ansible -o "/app/ansible${VERSION}" -- --help
}

function prep {
    local -ar requirements=( "ansible==${ANSIBLE}" "jmespath" "dnspython" "pymysql" "zipinfo" "podman-compose" )
    local -r extra_packages="${1}"

    printf "%s\n%s\n%s\n" "${requirements[@]}" > "${REQUIREMENTS}"

    if [ -f "/opt/${extra_packages}" ]; then
        cat "/opt/${extra_packages}" >> "${REQUIREMENTS}"
        rm -f "/opt/${extra_packages}"
    fi

    "/usr/bin/python${VERSION}" -m venv --clear /app/venv
    "/app/venv/bin/pip${VERSION}" install --quiet --upgrade pip pex wheel
}

prep "$(basename "${PACKAGES}")"
build_pex
