#!/usr/bin/env bash
# Shared test helpers for awn test scripts

RESULTS_FILE="$(mktemp)"
echo "0 0" > "$RESULTS_FILE"

assert_eq() {
  local label="$1" expected="$2" actual="$3"
  local pass fail
  read -r pass fail < "$RESULTS_FILE"
  if [ "$expected" = "$actual" ]; then
    echo "  PASS: $label"
    echo "$((pass + 1)) $fail" > "$RESULTS_FILE"
  else
    echo "  FAIL: $label — expected '$expected', got '$actual'"
    echo "$pass $((fail + 1))" > "$RESULTS_FILE"
  fi
}

assert_file_exists() {
  local label="$1" path="$2"
  local pass fail
  read -r pass fail < "$RESULTS_FILE"
  if [ -f "$path" ]; then
    echo "  PASS: $label"
    echo "$((pass + 1)) $fail" > "$RESULTS_FILE"
  else
    echo "  FAIL: $label — file '$path' does not exist"
    echo "$pass $((fail + 1))" > "$RESULTS_FILE"
  fi
}

print_results() {
  echo ""
  local pass fail
  read -r pass fail < "$RESULTS_FILE"
  echo "Results: $pass passed, $fail failed"
  [ "$fail" -eq 0 ] || exit 1
}
