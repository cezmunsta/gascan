# vim: ts=8:sw=8:ft=make:noai:noet
SHELL=/bin/bash

.PHONY: clean
.PHONY: prep
.PHONY: venv

# Options
ARCH?=amd64
BUILD_BASE?=quay.io/centos/centos:stream8
BUILD_DIR?=./build
ENTRYPOINT?=pmm-server.yaml
GO?=$(shell which go)
GOFMT?=$(shell which gofumpt 2>&1)
GOLINT?=$(shell which golint 2>&1)
NAME?=gascan
OS?=linux
PY?=3.8
VERSION?=$(shell git rev-parse HEAD)

# Constants
GIT_BRANCH_FILES:=$(shell (git diff-tree --no-commit-id --name-only -r main..HEAD; git diff --name-only --diff-filter=AM HEAD) | grep -F .go)
PACKAGE:=$(shell grep -E ^module go.mod | cut -f2 -d' ')
VETFLAGS=( -unusedresult -bools -copylocks -framepointer -httpresponse -json -stdmethods -printf -stringintconv -unmarshal -unsafeptr )

# Tests
DEBUG_BUILD:=$(shell test "${DEBUG}" = "1" && echo 1 || echo 0)
INSTALL_GO_FORMATTER:=$(shell test "${GOFMT/which: no/}" = "${GOFMT}" && echo 0 || echo 1)
INSTALL_GO_LINTER:=$(shell test "${GOLINT/which: no/}" = "${GOLINT}" && echo 0 || echo 1)
REQUIRES_GO_LINTING:=$(shell test "$(GIT_BRANCH_FILES)" = "" && echo 0 || echo 1)
#

init:
	@install -d "${BUILD_DIR}/${OS}/${ARCH}"
ifeq ($(INSTALL_GO_FORMATTER), 1)
	@cd ~ && "${GO}" install mvdan.cc/gofumpt@latest
endif
ifeq ($(INSTALL_GO_LINTER), 1)
	@cd ~ && "${GO}" install golang.org/x/lint/golint@latest
endif

all: ansible build 

ansible: ansible_image ansible_pex

ansible_image:
	@podman image rm "${NAME}-ansible:${VERSION}" || true
	@buildah bud -f images/ansible/Containerfile --build-arg BASE="${BUILD_BASE}" \
	  --squash --no-cache --force-rm --compress --tag "${NAME}-ansible:${VERSION}"

ansible_pex: prep
	@podman run --rm -it -v "${BUILD_DIR}/${OS}/${ARCH}":/app:Z "${NAME}-ansible:${VERSION}" "${PY}"
	@podman image rm "${NAME}-ansible:${VERSION}" || true
	@rm -rf "${BUILD_DIR}/${OS}/${ARCH}/venv"
	@cp -a "${BUILD_DIR}/${OS}/${ARCH}/ansible${PY}" "${BUILD_DIR}/ansible"

automation_lint:
	@venv/bin/ansible-lint --project-dir automation --write
	@git diff --exit-code --quiet $(GIT_BRANCH_FILES)

build: export GOOS=${OS}
build: export GOARCH=${ARCH}
build: export VNAME=${NAME}-py${PY}
build: build_prep pack check
	@go build -o "${BUILD_DIR}/${GOOS}/${GOARCH}/${VNAME}" -trimpath -race -gcflags="all=-N -l" -ldflags="-X main.EntryPointPlaybook=${ENTRYPOINT}"
	@cp -a "${BUILD_DIR}/${GOOS}/${GOARCH}/${VNAME}" "${BUILD_DIR}/gascan"
	@rm -vf "${BUILD_DIR}/ansible"

build_prep: export GOOS=${OS}
build_prep: export GOARCH=${ARCH}
build_prep: export VNAME=${NAME}-py${PY}
build_prep:
	@rm -vf "${BUILD_DIR}/gascan"
	@cp -a "${BUILD_DIR}/${GOOS}/${GOARCH}/ansible${PY}" "${BUILD_DIR}/ansible"

check: export GOOS=${OS}
check: export GOARCH=${ARCH}
ifeq ($(REQUIRES_GO_LINTING), 1)
check: go_lint go_fmt go_vet go_fix
else
check:
	@echo check: No Go files to check
endif

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

sample-bundle:
	@git archive --output=sample-bundle.tgz --format=tar.gz "${VERSION}" automation/{pmm-server-custom.yaml,ping.yaml,templates,roles,group_vars,host_vars}

venv:
	@python3 -m venv venv
	@venv/bin/pip install -U pip wheel
	@venv/bin/pip install -U ansible==5.6.0 jmespath dnspython ansible-lint
