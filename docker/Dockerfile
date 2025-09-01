FROM golang:1.23-alpine AS build
WORKDIR /app
RUN apk add --no-cache git ca-certificates

# Evita erro "error obtaining VCS status" quando não há .git no contexto
ENV GOFLAGS="-buildvcs=false"

# melhorar resiliência do proxy
RUN go env -w GOPROXY=https://proxy.golang.org,https://goproxy.io,direct

# Cache de deps: copia só manifests e baixa módulos
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compila (sem vendor)
#    - RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 >>>> binário estático (alpine leve no runtime)
#    - go build -trimpath -ldflags="-s -w" >>>>>> binário menor
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /bin/api ./cmd/api


FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /home/appuser

COPY --from=build /bin/api /usr/local/bin/api

# shutdown mais previsível
STOPSIGNAL SIGTERM

EXPOSE 8080
USER appuser
ENTRYPOINT ["/usr/local/bin/api"]
