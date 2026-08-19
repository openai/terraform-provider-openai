#!/usr/bin/env bash

# Source this policy before running Go or Git, or exposing release credentials.
trusted_release_environment=(
  'GOENV=off'
  'GOFLAGS='
  'GOCACHEPROG='
  'GOWORK=off'
  'GOTOOLCHAIN=local'
  'GO111MODULE=on'
  'CGO_ENABLED=0'
  'GOEXPERIMENT='
  'GOFIPS140=off'
  'GODEBUG='
  'GOTMPDIR='
  'GOAUTH=netrc'
  'GCCGO='
  'GCCGOTOOLDIR='
  'CC='
  'CXX='
  'FC='
  'AR='
  'PKG_CONFIG='
  'GIT_CONFIG_GLOBAL=/dev/null'
  'GIT_CONFIG_SYSTEM=/dev/null'
  'GIT_CONFIG_NOSYSTEM=1'
  'GIT_CONFIG_PARAMETERS='
  'GIT_CONFIG_COUNT=5'
  'GIT_CONFIG_KEY_0=core.fsmonitor'
  'GIT_CONFIG_VALUE_0=false'
  'GIT_CONFIG_KEY_1=core.hooksPath'
  'GIT_CONFIG_VALUE_1=/dev/null'
  'GIT_CONFIG_KEY_2=diff.external'
  'GIT_CONFIG_VALUE_2='
  'GIT_CONFIG_KEY_3=credential.helper'
  'GIT_CONFIG_VALUE_3='
  'GIT_CONFIG_KEY_4=core.askPass'
  'GIT_CONFIG_VALUE_4='
  'GIT_EXTERNAL_DIFF='
  'GIT_ASKPASS='
  'GIT_SSH='
  'GIT_SSH_COMMAND='
  'GIT_PAGER=cat'
  'GIT_EDITOR=false'
  'GIT_SEQUENCE_EDITOR=false'
  'GIT_TERMINAL_PROMPT=0'
)

# Inherited roots can redirect otherwise trusted Go and Git executables.
unset GOROOT GOTOOLDIR GIT_EXEC_PATH

for assignment in "${trusted_release_environment[@]}"; do
  export "$assignment"
done

trusted_go_root="$(go env GOROOT)"
test -n "$trusted_go_root"
export GOROOT="$trusted_go_root"
trusted_release_environment+=("GOROOT=$GOROOT")

trusted_git_exec_path="$(git --exec-path)"
test -d "$trusted_git_exec_path"
export GIT_EXEC_PATH="$trusted_git_exec_path"
trusted_release_environment+=("GIT_EXEC_PATH=$GIT_EXEC_PATH")

for assignment in "${trusted_release_environment[@]}"; do
  printf '%s\n' "$assignment" >> "${GITHUB_ENV:?GITHUB_ENV must be set}"
done
