#!/usr/bin/env bash
set -euo pipefail

FORMULA="${DOQ_HOMEBREW_FORMULA:-aayush9029/tap/doq}"
SYMBOL_QUERY="${DOQ_SYMBOL_QUERY:-URL}"
DOC_QUERY="${DOQ_DOC_QUERY:-swift testing}"
DOC_IDENTIFIER="${DOQ_DOC_IDENTIFIER:-/documentation/Testing}"
INDEX_FRAMEWORKS="${DOQ_INDEX_FRAMEWORKS:-Swift Foundation}"

if ! command -v brew >/dev/null 2>&1; then
  echo "brew is required" >&2
  exit 1
fi

require_vector_prereqs() {
  local macos_major
  macos_major="$(sw_vers -productVersion | cut -d. -f1)"
  if [[ "${macos_major}" -lt 26 ]]; then
    echo "semantic docs E2E requires macOS 26+" >&2
    exit 1
  fi
  if [[ ! -d /System/Library/PrivateFrameworks/VectorSearch.framework ]]; then
    echo "VectorSearch.framework is missing" >&2
    exit 1
  fi
  if [[ ! -d /System/Library/AssetsV2/com_apple_MobileAsset_AppleDeveloperDocumentation ]]; then
    echo "AppleDeveloperDocumentation asset root is missing" >&2
    exit 1
  fi
}

echo "==> Installing ${FORMULA}"
brew install "${FORMULA}"

BIN="$(brew --prefix doq)/bin/doq"
if [[ ! -x "${BIN}" ]]; then
  echo "installed doq binary not found at ${BIN}" >&2
  exit 1
fi

TMP_HOME="$(mktemp -d)"
trap 'rm -rf "${TMP_HOME}"' EXIT

read -r -a FRAMEWORK_ARGS <<< "${INDEX_FRAMEWORKS}"

echo "==> Verifying installed binary"
"${BIN}" --version

echo "==> Exercising regular SQLite index/search"
HOME="${TMP_HOME}" "${BIN}" index "${FRAMEWORK_ARGS[@]}"
HOME="${TMP_HOME}" "${BIN}" search "${SYMBOL_QUERY}" >/dev/null
HOME="${TMP_HOME}" "${BIN}" info "${SYMBOL_QUERY}" >/dev/null

echo "==> Exercising vector docs backend"
require_vector_prereqs
HOME="${TMP_HOME}" "${BIN}" docs search "${DOC_QUERY}" --limit 3 >/dev/null
HOME="${TMP_HOME}" "${BIN}" docs get "${DOC_IDENTIFIER}" --json >/dev/null

echo "Homebrew E2E passed"
