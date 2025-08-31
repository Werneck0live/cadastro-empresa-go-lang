# FROM golang:1.23-alpine AS build
# WORKDIR /app
# RUN apk add --no-cache git ca-certificates

# COPY . .
# #força usar vendor
# RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o /bin/api ./cmd/api

# FROM alpine:3.20
# RUN adduser -D -u 10001 appuser
# WORKDIR /home/appuser
# COPY --from=build /bin/api /usr/local/bin/api
# EXPOSE 8080
# USER appuser
# ENTRYPOINT ["/usr/local/bin/api"]


# docker/Dockerfile


######## DEIXANDO ESSA VERSÃO DO DOCKERFILE ABAIXO, POR ENQAUNTO, NO INTUITO 
######## DE USAR -mod=vendor, PORQUE PRECISO DELE PARA PODER VERIFICAR
######## AS ALTERAÇÕES DO CÓDIGO EM TEMPO REAAL -

    ######## TODO: Retirar a versão do Dockerfile abaixo e retornar
    ######## pra versão de cima



FROM golang:1.23-alpine AS build
WORKDIR /app
RUN apk add --no-cache git ca-certificates

# Evita erro de VCS stamping em ambientes sem .git
ENV GOFLAGS="-buildvcs=false"

# Copia o módulo inteiro
COPY . .

# Opcional: proxies redundantes evitam falhas intermitentes
RUN go env -w GOPROXY=https://proxy.golang.org,https://goproxy.io,direct

# Gera/atualiza vendor com base no go.mod/go.sum copiados
RUN go mod tidy && go mod vendor

# Build usando vendor
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -mod=vendor -trimpath -o /bin/api ./cmd/api

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /home/appuser
COPY --from=build /bin/api /usr/local/bin/api
EXPOSE 8080
USER appuser
ENTRYPOINT ["/usr/local/bin/api"]
