#!/usr/bin/env bash

# Source this policy before running Go or exposing release credentials.
trusted_go_environment=(
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
)

# An inherited GOROOT can redirect the otherwise trusted Go executable.
unset GOROOT GOTOOLDIR

for assignment in "${trusted_go_environment[@]}"; do
  export "$assignment"
done

trusted_go_root="$(go env GOROOT)"
test -n "$trusted_go_root"
export GOROOT="$trusted_go_root"
trusted_go_environment+=("GOROOT=$GOROOT")

for assignment in "${trusted_go_environment[@]}"; do
  printf '%s\n' "$assignment" >> "${GITHUB_ENV:?GITHUB_ENV must be set}"
done
