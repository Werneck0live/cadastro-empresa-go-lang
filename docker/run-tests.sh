#!/usr/bin/env bash
set -euo pipefail

# Reforçando para evitar o erro de VCS stamping no container 
# (também tem essa configuração do docker-compose.yml - GOFLAGS: -buildvcs=false)
export GOFLAGS="${GOFLAGS:-} -buildvcs=false"

# garante 'go' no PATH
command -v go >/dev/null 2>&1 || export PATH="/usr/local/go/bin:${PATH}"

# helpers
shortpkg () { awk -F/ '{n=NF; if(n>=2){print $(n-1)"/"$n} else {print $0}}'; }

emit_no_tests_line () {
  # Emite uma “linha JSON” (fake) para marcar pacote sem testes
  local pkg="$1"
  echo "{\"Package\":\"$pkg\",\"Test\":\"(no tests)\",\"Action\":\"skip\"}"
}

print_tests_by_name () {
  # Imprime: "TestName: OK/ERROR/SKIPPED" (ou "( pkg ) no tests: SKIPPED")
  local json="$1" title="$2"
  echo "==> $title"
  jq -rR '
    fromjson? | select(.) |
    select(.Test != null and (.Action=="pass" or .Action=="fail" or .Action=="skip")) |
    if .Test == "(no tests)" then
      "( " + (.Package | (split("/") | (.[-2:] // .) | join("/"))) + " ) no tests: SKIPPED"
    else
      .Test + ": " + (if .Action=="pass" then "OK"
                      elif .Action=="fail" then "ERROR"
                      else "SKIPPED" end)
    end
  ' "$json" | sort
  echo
}

print_summary_counts () {
  # Resumo agregado OK/ERROR/SKIPPED
  local json="$1" title="$2"
  echo "Resumo $title:"
  jq -rR '
    fromjson? | select(.) |
    select(.Test != null and (.Action=="pass" or .Action=="fail" or .Action=="skip")) |
    if .Action=="pass" then "OK"
    elif .Action=="fail" then "ERROR"
    else "SKIPPED" end
  ' "$json" | sort | uniq -c | awk '{printf "  %-7s %d\n", $2, $1}'
  echo
}

run_pack_set () {
  # Roda uma lista de pacotes e junta JSON em OUT_JSON (2º argumento), SEM engolir stdout
  local title="$1"; shift
  local out_json="$1"; shift
  local pkgs=("$@")
  : >"$out_json"
  local EXIT_LOCAL=0

  for pkg in "${pkgs[@]}"; do
    tmp="$(mktemp)"
    if go test -json -race "$pkg" -count=1 >"$tmp" 2>&1; then
      :
    else
      EXIT_LOCAL=1
    fi
    # Se não houve nenhum evento de teste, marca como “no tests: SKIPPED”
    if ! jq -e -rR 'fromjson? | select(.) | select(.Test != null) | .Test' "$tmp" >/dev/null 2>&1; then
      emit_no_tests_line "$pkg" >>"$tmp"
    fi
    cat "$tmp" >>"$out_json"
  done

  print_tests_by_name "$out_json" "$title"
  print_summary_counts "$out_json" "$title"
  return $EXIT_LOCAL
}

merge_jsons () { local out="$1"; shift; : >"$out"; for f in "$@"; do [ -f "$f" ] && cat "$f" >>"$out"; done; }

# execução
EXIT_ALL=0

# 1) UNIT — roda TODOS os pacotes do módulo (mesmo sem _test.go)
ALL_PKGS=($(go list ./...))
UNIT_JSON="$(mktemp)"
run_pack_set "Unit tests" "$UNIT_JSON" "${ALL_PKGS[@]}" || EXIT_ALL=1

# 2) INTEGRAÇÃO — somente os pacotes de integração (se RUN_INT=1)
INT_REPO_JSON=""
INT_BROKER_JSON=""
if [ "${RUN_INT:-1}" = "1" ]; then
  INT_REPO_JSON="$(mktemp)"
  run_pack_set "Integration tests (repository)" "$INT_REPO_JSON" ./internal/repository || EXIT_ALL=1

  INT_BROKER_JSON="$(mktemp)"
  run_pack_set "Integration tests (broker)" "$INT_BROKER_JSON" ./internal/broker || EXIT_ALL=1
else
  echo "==> Integration tests SKIPPED (RUN_INT=${RUN_INT:-unset})"
fi

# 3) FINAL — junta tudo e mostra contagem total
FINAL_JSON="$(mktemp)"
merge_jsons "$FINAL_JSON" "$UNIT_JSON" "$INT_REPO_JSON" "$INT_BROKER_JSON"
echo "==> FINAL"
print_summary_counts "$FINAL_JSON" "geral"

exit $EXIT_ALL
