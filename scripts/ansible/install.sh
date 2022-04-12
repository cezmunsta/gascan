#!/bin/bash -x
# buildah bud -f images/ansible/Containerfile --squash --no-cache --force-rm --compress --tag msp-ansible
# podman run --rm -it -v ./:/app
# podman run --rm -it -v ./:/app 3.9
# PEX_UNZIP=1 PEX_ROOT=/tmp/foo ./ansible3.8 --help
set -eu

declare VERSION="${1:-3.8}"
declare ANSIBLE="${2:-5.6.0}"
declare -r REQUIREMENTS='/tmp/requirements.txt'

function build_pex {
    /app/venv/bin/pex -r "${REQUIREMENTS}" -c ansible -o "/app/ansible${VERSION}" -- --help
}

cat <<EOS > "${REQUIREMENTS}"
ansible==${ANSIBLE}
jmespath
dnspython
EOS

"/usr/bin/python${VERSION}" -m venv --clear /app/venv
"/app/venv/bin/pip${VERSION}" install --quiet --upgrade pip pex wheel

build_pex
