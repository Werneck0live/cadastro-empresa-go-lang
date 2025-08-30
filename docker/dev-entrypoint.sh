#!/usr/bin/env sh
set -eu

# garante que 'go' e o binário do air fiquem no PATH
export PATH="/usr/local/go/bin:/go/bin:$PATH"
# baixa automaticamente um toolchain de Go mais novo quando necessário
export GOTOOLCHAIN=auto

echo "[entrypoint] PWD=$(pwd)"
echo "[entrypoint] PATH=$PATH"

if [ ! -f /app/docker/dev-entrypoint.sh ]; then
  echo "[entrypoint][FATAL] /app/docker/dev-entrypoint.sh não encontrado (bind mount ..:/app?)"
  exec sleep infinity
fi
if [ ! -f /app/docker/.air.toml ]; then
  echo "[entrypoint][FATAL] /app/docker/.air.toml não encontrado."
  exec sleep infinity
fi

echo "[entrypoint] Instalando pacotes (git/ca-certificates)..."
apk add --no-cache git ca-certificates >/dev/null

go version || true

# instala Air do módulo correto (air-verse); com GOTOOLCHAIN=auto o Go baixa o toolchain suportado
echo "[entrypoint] Instalando Air (github.com/air-verse/air@latest)..."
if ! go install github.com/air-verse/air@latest 2>&1; then
  echo "[entrypoint][FATAL] falha ao instalar o Air."
  exec sleep infinity
fi

AIR_BIN="$(go env GOPATH)/bin/air"
if [ ! -x "$AIR_BIN" ]; then
  echo "[entrypoint][FATAL] Air não encontrado em $AIR_BIN."
  exec sleep infinity
fi

mkdir -p /app/tmp

if [ ! -d /app/cmd/api ]; then
  echo "[entrypoint][FATAL] diretório /app/cmd/api não existe."
  exec sleep infinity
fi

echo "[entrypoint] Air versão:"
"$AIR_BIN" -v || true

echo "[entrypoint] Iniciando Air..."
exec "$AIR_BIN" -c /app/docker/.air.toml
