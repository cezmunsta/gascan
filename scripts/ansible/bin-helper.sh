#!/bin/bash
#
# This is a helper script for use with an Ansible PEX
# so that the standard Ansible commands can be easily
# used.
#
# This requires symlinks to be created, e.g.
#
# for l in ansible ansible-playbook ansible-config ansible-vault ansible-inventory; do
#     ln -s "${HOME}"/bin/ansible.sh "${HOME}"/bin/"${l}";
# done
#
set -eu

PEX_SCRIPT="$(basename "${0}")"

case "${PEX_SCRIPT}" in
	ansible | ansible-vault | ansible-playbook | ansible-inventory | ansible-config ) export PEX_SCRIPT;;
        *) echo "ERROR: unsupported item '${PEX_SCRIPT}'"; exit 1
esac

"${HOME}"/bin/ansible.pex "${@}"
