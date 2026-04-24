#!/usr/bin/env bash
# Smoke-test the running translator by hitting the HTTP endpoint with a fixed
# set of chat messages that exercise the prompt rules (proper-noun handling,
# target-language enforcement, skip behaviour, passthrough, etc).
#
# Usage:
#   test/run_cases.sh                         # defaults: http://127.0.0.1:5000/wowschat/
#   test/run_cases.sh -u http://host:port/path
#   HOST=127.0.0.1 PORT=5000 PATH_=/wowschat/ test/run_cases.sh
#
# Prereq: start the translator separately (it must already be listening).

set -u

HOST="${HOST:-127.0.0.1}"
PORT="${PORT:-5000}"
PATH_="${PATH_:-/wowschat/}"
URL=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -u|--url) URL="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,12p' "$0"
      exit 0
      ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

if [[ -z "$URL" ]]; then
  URL="http://${HOST}:${PORT}${PATH_}"
fi

CASES=(
  # A. Proper-noun rule (lowercase ship/callsign + normal sentence)
  "furious are u fast?"
  "yamato push B"
  "bismarck go cap"
  "montana is low hp"

  # B. Over-application check (common words that could look like ship/map names)
  "fast ship coming"
  "furious push now"
  "push mid now"

  # C. Target-language compliance
  "我被卡住了"
  "thx bro"

  # D. Skip-rule tuning
  "asdfgh"
  "brb afk 2min"

  # E. Passthrough / placeholder behaviour
  "gg wp"
  "RPF: Montana"
)

urlencode() {
  # Rely on jq for robust URL-encoding; fall back to python3 if jq missing.
  if command -v jq >/dev/null 2>&1; then
    jq -rn --arg s "$1" '$s|@uri'
  else
    python3 -c 'import sys,urllib.parse;print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"
  fi
}

printf 'endpoint: %s\n' "$URL"
printf '%s\n' "--------------------------------"

for text in "${CASES[@]}"; do
  encoded=$(urlencode "$text")
  printf '> %s\n' "$text"
  # -s silent, -S show errors, --max-time bounds each request
  body=$(curl -sS --max-time 30 --get --data-urlencode "text=$text" "$URL" || true)
  if [[ -z "$body" ]]; then
    printf '  (no translation / skipped)\n'
  else
    # Indent each line of the response for readability
    printf '%s\n' "$body" | sed 's/^/  /'
  fi
  printf '\n'
done
