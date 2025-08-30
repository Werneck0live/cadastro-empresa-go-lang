FROM golang:1.23-alpine AS build
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY . .
#força usar vendor
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o /bin/api ./cmd/api

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /home/appuser
COPY --from=build /bin/api /usr/local/bin/api
EXPOSE 8080
USER appuser
ENTRYPOINT ["/usr/local/bin/api"]
