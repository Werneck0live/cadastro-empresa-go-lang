# docker/Dockerfile.ws
FROM golang:1.23-alpine AS build
WORKDIR /app
RUN apk add --no-cache git ca-certificates

# Evita erro de VCS stamping (quando não há .git no build)
ENV GOFLAGS="-buildvcs=false"

# 1) Copia só os manifests (melhor aproveitamento de cache)
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://proxy.golang.org,https://goproxy.io,direct \
 && go mod download

# 2) Copia o restante do código (inclui cmd/ws e internal/ws)
COPY . .

# 3) Gera/atualiza vendor (você quer manter -mod=vendor por enquanto)
#RUN go mod tidy && go mod vendor
RUN set -eux; pwd; ls -la; test -f go.mod || (echo "MISSING go.mod" && exit 1); \
    ls -la cmd || true; ls -la cmd/ws || true


# 4) Compila usando vendor
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -mod=vendor -trimpath -o /bin/ws ./cmd/ws

# Runtime
FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /home/appuser
COPY --from=build /bin/ws /usr/local/bin/ws
USER appuser
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/ws"]
