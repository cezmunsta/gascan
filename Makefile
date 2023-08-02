# vim: ts=8:sw=8:ft=make:noai:noet
SHELL=/bin/bash

.PHONY: clean
.PHONY: prep
.PHONY: test
.PHONY: venv

# Options
ANSIBLE?=6.7.0
ARCH?=amd64
AUTH_FIELD_1?=Auth-Id
AUTH_FIELD_2?=Auth-Token
AUTH_FIELD_3?=Monitor-Name
BUILD_BASE?=quay.io/centos/centos:stream8
BUILD_DIR?=./build
BUNDLE_VERSION?=$(shell git rev-parse HEAD)
ENTRYPOINT?=pmm-full.yaml
EXTRACT_ANSIBLE_CONFIG?=0
EXTRACT_DYNAMIC_INVENTORY?=1
GO?=$(shell which go)
GOFMT?=$(shell which gofumpt 2>&1)
GOLINT?=$(shell which golint 2>&1)
NAME?=gascan
OS?=linux
PACKAGES_OS?=images/ansible/extra_packages_os.txt
PACKAGES_PIP?=images/ansible/extra_packages_pip.txt
PY?=3.9
VERSION?=$(shell git rev-parse HEAD)

# Constants
BUILD_BASE_TAG:=$(shell echo "${BUILD_BASE}" | sed 's^.*/^^g; s/:/-/g' | cut -f2 -d'/')
GIT_BRANCH_FILES:=$(shell (git diff-tree --no-commit-id --name-only -r main..HEAD; git diff --name-only --diff-filter=AM HEAD) | grep -F .go)
PACKAGE:=$(shell grep -E ^module go.mod | cut -f2 -d' ')
VETFLAGS=( -unusedresult -bools -copylocks -framepointer -httpresponse -json -stdmethods -printf -stringintconv -unmarshal -unsafeptr )

# Tests
DEBUG_BUILD:=$(shell test "${DEBUG}" = "1" && echo 1 || echo 0)
INSTALL_GO_FORMATTER:=$(shell test "${GOFMT/which: no/}" = "${GOFMT}" && echo 0 || echo 1)
INSTALL_GO_LINTER:=$(shell test "${GOLINT/which: no/}" = "${GOLINT}" && echo 0 || echo 1)
REQUIRES_GO_LINTING:=$(shell test "$(GIT_BRANCH_FILES)" = "" && echo 0 || echo 1)
#

init: prep
init:
ifeq ($(INSTALL_GO_FORMATTER), 1)
	@cd ~ && "${GO}" install mvdan.cc/gofumpt@latest
endif
ifeq ($(INSTALL_GO_LINTER), 1)
	@cd ~ && "${GO}" install golang.org/x/lint/golint@latest
endif

all: ansible build 

all_versions:
	@printf "all_el9\nall_el8\nall_el7\nall_jammy\nall_bullseye"

all_el9: export BUILD_BASE=quay.io/centos/centos:stream9
all_el9: export BUILD_BASE_TAG=centos-stream9
all_el9: export PY=3.9
all_el9: ansible build

all_el8: export BUILD_BASE=quay.io/centos/centos:stream8
all_el8: export BUILD_BASE_TAG=centos-stream8
all_el8: export PY=3.9
all_el8: ansible build

all_el7: export BUILD_BASE=quay.io/centos/centos:7
all_el7: export BUILD_BASE_TAG=centos-7
all_el7: export PY=3.8
all_el7: ansible build

all_jammy: export BUILD_BASE=ubuntu:jammy
all_jammy: export BUILD_BASE_TAG=ubuntu-jammy
all_jammy: export PY=3.10
all_jammy: ansible build

all_bullseye: export BUILD_BASE=debian:bullseye
all_bullseye: export BUILD_BASE_TAG=debian-bullseye
all_bullseye: export PY=3.9
all_bullseye: ansible build

ansible: ansible_image ansible_pex

ansible_image: export VNAME=${NAME}/${BUILD_BASE_TAG}-ansible:${VERSION}
ansible_image:
	@podman image exists "${VNAME}" && podman image rm "${VNAME}" || true
	@buildah bud -f images/ansible/Containerfile --pull \
	  --build-arg BASE="${BUILD_BASE}" \
	  --build-arg PACKAGES_OS="${PACKAGES_OS}" --build-arg PACKAGES_PIP="${PACKAGES_PIP}" \
	  --build-arg ANSIBLE="${ANSIBLE}" --squash --no-cache --force-rm --compress --tag "${VNAME}"

ansible_pex: export VDIR=${BUILD_DIR}/${OS}/${ARCH}/${BUILD_BASE_TAG}
ansible_pex: export VNAME=${NAME}/${BUILD_BASE_TAG}-ansible:${VERSION}
ansible_pex: prep
	@podman run --rm -it -v "${VDIR}":/app:Z "${VNAME}" "${PY}" "${ANSIBLE}" "${PACKAGES_PIP}"
	@rm -rf "${VDIR}/venv"
	@cp -a "${VDIR}/ansible${PY}" "${BUILD_DIR}/ansible"

automation_lint:
	@venv/bin/ansible-lint --project-dir automation --write
	@git diff --exit-code --quiet $(GIT_BRANCH_FILES)

build: export GOOS=${OS}
build: export GOARCH=${ARCH}
build: export VDIR=${BUILD_DIR}/${OS}/${ARCH}/${BUILD_BASE_TAG}
build: export VNAME=${VDIR}/${NAME}-py${PY}
build: export CGO_ENABLED=0
build: build_prep check
ifeq ($(DEBUG_BUILD), 1)
	@${GO} build -o "${VNAME}" -trimpath -gcflags="all=-N -l" \
		-ldflags="-X main.EntryPointPlaybook=${ENTRYPOINT} -X main.HeaderIdentifier=${AUTH_FIELD_1} -X main.HeaderToken=${AUTH_FIELD_2} -X main.HeaderMonitorName=${AUTH_FIELD_3}"
else
	@${GO} build -o "${VNAME}" -trimpath \
		-ldflags="-s -w -X main.EntryPointPlaybook=${ENTRYPOINT} -X main.HeaderIdentifier=${AUTH_FIELD_1} -X main.HeaderToken=${AUTH_FIELD_2} -X main.HeaderMonitorName=${AUTH_FIELD_3}"
endif
	@cp -a "${VNAME}" "${BUILD_DIR}/gascan"
	@rm -f version.go

build_prep: export GOOS=${OS}
build_prep: export GOARCH=${ARCH}
build_prep: export VDIR=${BUILD_DIR}/${OS}/${ARCH}/${BUILD_BASE_TAG}
build_prep: export VNAME=${VDIR}/ansible${PY}
build_prep: pack go_generate
	@rm -vf "${BUILD_DIR}/gascan"
	@cp -a "${VNAME}" "${BUILD_DIR}/ansible"

check: export GOOS=${OS}
check: export GOARCH=${ARCH}
ifeq ($(REQUIRES_GO_LINTING), 1)
check: go_lint go_fmt go_vet go_fix
else
check:
	@echo check: No Go files to check
endif

test: export EXTRACT_ANSIBLE_CONFIG=1
test: export EXTRACT_DYNAMIC_INVENTORY=1
test: export GASCAN_TEST_NOEXIT=1
test: go_generate check
	@${GO} test ./...
	@rm -f version.go

go_generate: export ANSIBLE_VERSION="${ANSIBLE}"
go_generate: export BUNDLE_RELEASE_VERSION="${BUNDLE_VERSION}"
go_generate: export PYTHON_VERSION="${PY}"
go_generate: export RELEASE_VERSION="${VERSION}"
go_generate:
	@${GO} generate

go_fix: export PACKAGE=./
go_fix:
	@"${GO}" tool fix -diff "${PACKAGE}"
	@git diff --exit-code --quiet $(GIT_BRANCH_FILES)

go_fmt:
	@"${GOFMT}" -w -e $(GIT_BRANCH_FILES)
	@git diff --exit-code --quiet $(GIT_BRANCH_FILES)

go_lint:
	@"${GOLINT}" -set_exit_status "${PACKAGE}"

go_vet:
	@"${GO}" vet "${VETFLAGS[@]}" "${PACKAGE}"

clean:
	@find "${BUILD_DIR}" -type f -print -delete
	@rm -vf bundle.tgz
	@rm -vrf venv
	@rm -f version.go
	@"${GO}" clean -x
	@"${GO}" clean -x -cache
	@"${GO}" clean -x -testcache

pack:
ifndef BUNDLE
	@echo Exporting bundle
	@git archive --output=bundle.tgz --format=tar.gz "${VERSION}" automation
else
	@echo Copying custom bundle "${BUNDLE}"
	@cp -a "${BUNDLE}" bundle.tgz
endif

prep:
	@install -d "${BUILD_DIR}/${OS}/${ARCH}/${BUILD_BASE_TAG}"

sample-bundle:
	@git archive --output=sample-bundle.tgz --format=tar.gz "${VERSION}" automation/{pmm-server-custom.yaml,ping.yaml,templates,roles,group_vars,host_vars} scripts/dynamic-inventory/get_inventory.py scripts/connect/connect.py

venv:
	@python3 -m venv venv
	@venv/bin/pip install -U pip wheel
	@venv/bin/pip install -U ansible=="${ANSIBLE}" jmespath dnspython ansible-lint
