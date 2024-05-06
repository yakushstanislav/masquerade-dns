.PHONY: build build-linux build-windows build-macos lint lint-fix
.DEFAULT_GOAL := build

PWD = $(shell pwd)

BUILD_DIR = ${PWD}/bin

CMDS = server

VERSION = 1.0

build: build-linux build-windows build-macos

build-linux:
	@$(foreach cmd, $(CMDS), APP_NAME=${cmd} APP_VERSION=${VERSION} APP_DIR=${PWD}/cmd/${cmd} BUILD_DIR=${BUILD_DIR} OS=linux ARCH=amd64 ./scripts/build.sh || exit;)

build-windows:
	@$(foreach cmd, $(CMDS), APP_NAME=${cmd} APP_VERSION=${VERSION} APP_DIR=${PWD}/cmd/${cmd} BUILD_DIR=${BUILD_DIR} OS=windows ARCH=amd64 ./scripts/build.sh || exit;)

build-macos:
	@$(foreach cmd, $(CMDS), APP_NAME=${cmd} APP_VERSION=${VERSION} APP_DIR=${PWD}/cmd/${cmd} BUILD_DIR=${BUILD_DIR} OS=darwin ARCH=arm64 ./scripts/build.sh || exit;)

clean:
	@rm -rf $(BUILD_DIR)

lint:
	@golangci-lint run

lint-fix:
	@golangci-lint run --fix