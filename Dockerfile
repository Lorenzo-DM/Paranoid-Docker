FROM oven/bun:1 AS frontend
WORKDIR /app/webui
COPY webui/package.json webui/bun.lock ./
RUN bun install --frozen-lockfile
COPY webui/ .
RUN bun run build

FROM golang:1.26-alpine3.22 AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /server ./cmd/server
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /paranoid ./cmd/paranoid

FROM alpine:3.22
RUN apk add --no-cache docker-cli docker-cli-compose ca-certificates tzdata
WORKDIR /app

COPY --from=backend /server .
COPY --from=backend /paranoid /usr/local/bin/paranoid
COPY --from=frontend /app/webui/dist ./web

RUN mkdir -p rollbacks images

ENV WEB_DIR=/app/web
ENV LISTEN_ADDR=:1323

EXPOSE 1323

ENTRYPOINT ["./server"]
