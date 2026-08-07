# syntax=docker/dockerfile:1
FROM nginx:1.27-alpine AS certs

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/urock-api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/urock-admin ./cmd/admin

FROM scratch
WORKDIR /app
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/urock-api /app/urock-api
COPY --from=build /out/urock-admin /app/urock-admin
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/urock-api"]
