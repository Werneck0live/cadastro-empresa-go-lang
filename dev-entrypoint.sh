#!/usr/bin/env sh
set -eu

# garante que 'go' e o binário do air fiquem no PATH
export PATH="/usr/local/go/bin:/go/bin:$PATH"
# baixa automaticamente um toolchain de Go mais novo quando necessário (ex.: Air pede Go >= 1.24)
export GOTOOLCHAIN=auto

echo "[entrypoint] PWD=$(pwd)"
echo "[entrypoint] PATH=$PATH"

# checa arquivos essenciais
if [ ! -f /app/dev-entrypoint.sh ]; then
  echo "[entrypoint][FATAL] /app/dev-entrypoint.sh não encontrado. Verifique o bind mount (.:/app)."
  exec sleep infinity
fi
if [ ! -f /app/.air.toml ]; then
  echo "[entrypoint][FATAL] /app/.air.toml não encontrado na raiz do projeto."
  exec sleep infinity
fi

echo "[entrypoint] Instalando pacotes (git/ca-certificates)..."
apk add --no-cache git ca-certificates >/dev/null

# informa versão local do Go (a ferramenta pode baixar outra com GOTOOLCHAIN=auto)
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

# cria pasta tmp (saída do build do Air)
mkdir -p /app/tmp

# valida diretório do main
if [ ! -d /app/cmd/api ]; then
  echo "[entrypoint][FATAL] diretório /app/cmd/api não existe (onde fica seu main.go)."
  exec sleep infinity
fi

echo "[entrypoint] Air versão:"
"$AIR_BIN" -v || true

echo "[entrypoint] Iniciando Air..."
exec "$AIR_BIN" -c /app/.air.toml
