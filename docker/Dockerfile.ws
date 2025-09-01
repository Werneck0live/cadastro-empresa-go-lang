FROM golang:1.23-alpine AS build
WORKDIR /app
RUN apk add --no-cache git ca-certificates
ENV GOFLAGS="-buildvcs=false"

# proxy resiliente
RUN go env -w GOPROXY=https://proxy.golang.org,https://goproxy.io,direct

# cache de módulos
COPY go.mod go.sum ./
RUN go mod download

# código
COPY . .

RUN set -eux; \
    go version; \
    test -f go.mod && echo "ok go.mod"; \
    ls -la cmd/ws || true; \
    echo "== go list -m all =="; go list -m all | head -n 50; \
    echo "== go build -x -v =="; \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -x -v -o /bin/ws ./cmd/ws

# 🔧 se algum build antigo deixou vendor/ dentro da imagem, remove
RUN rm -rf vendor

# build ignorando vendor explicitamente
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -mod=mod -trimpath -ldflags="-s -w" \
    -o /bin/ws ./cmd/ws

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /home/appuser
COPY --from=build /bin/ws /usr/local/bin/ws
STOPSIGNAL SIGTERM
EXPOSE 8090
USER appuser
ENTRYPOINT ["/usr/local/bin/ws"]
