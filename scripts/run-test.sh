#!/usr/bin/env bash
set -euo pipefail

TEST_NAME="${1:-}"
if [[ -z "$TEST_NAME" ]]; then
  echo "usage: $0 <test_name>" >&2
  exit 2
fi

# Allow Makefile/env to override; provide defaults when run directly.
MAELSTROM_DIR="${MAELSTROM_DIR:-./bin/maelstrom}"
BINARY="${BINARY:-./bin/gossip-glomers}"
VERBOSE="${VERBOSE:-0}"

mkdir -p test-run
timestamp="$(date +%Y%m%d-%H%M%S)"
log="test-run/${timestamp}.log"

# Colors
green=$'\033[32m'
red=$'\033[31m'
cyan=$'\033[36m'
reset=$'\033[0m'
dim=$'\033[2m'

format_duration() {
  local secs=$1
  printf "%02d:%02d.%02d" $((secs/60)) $((secs%60)) 0
}

start=$(date +%s)

case "$TEST_NAME" in
  echo)
    cmd=( "${MAELSTROM_DIR}/maelstrom" test -w echo --bin "${BINARY}" --node-count 1 --time-limit 10 )
    ;;
  unique-ids)
    cmd=( "${MAELSTROM_DIR}/maelstrom" test -w unique-ids \
          --bin "${BINARY}" \
          --time-limit 30 \
          --rate 1000 \
          --node-count 3 \
          --availability total \
          --nemesis partition )
    ;;
  *)
    echo "unknown test: ${TEST_NAME}" >&2
    exit 2
    ;;
esac

set +e
if [[ "$VERBOSE" -eq 1 ]]; then
  # Verbose: stream everything and log it.
  "${cmd[@]}" | tee "${log}"
  code=${PIPESTATUS[0]}
else
  # Silent + spinner-only UI.
  "${cmd[@]}" >"${log}" 2>&1 &
  pid=$!

  # Smooth braille spinner
  frames=( "⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏" )
  trap 'tput cnorm 2>/dev/null || true; printf "\r\033[K" >&2' EXIT
  tput civis 2>/dev/null || true

  i=0
  while kill -0 "$pid" 2>/dev/null; do
    printf "\r%s%s%s" "$cyan" "${frames[i % ${#frames[@]}]}" "$reset" >&2
    i=$((i+1))
    sleep 0.08
  done

  wait "$pid"
  code=$?

  # Clear spinner
  printf "\r\033[K" >&2
  tput cnorm 2>/dev/null || true
  trap - EXIT
fi
set -e

end=$(date +%s)
dur=$((end - start))
success_str=$'Everything looks good! ヽ(‘ー`)ノ'

if [[ $code -eq 0 ]] && grep -Fq "$success_str" "${log}"; then
  printf "%sOK%s (%s)  %s\n" "$green" "$reset" "$(format_duration "$dur")" "${dim}${log}${reset}"
  exit 0
else
  printf "%sFAILED%s (%s)  %s\n" "$red" "$reset" "$(format_duration "$dur")" "${dim}${log}${reset}"
  exit 1
fi
