# syntax=docker/dockerfile:1

# ── Frontend ─────────────────────────────────────────────
FROM node:22-alpine AS web
WORKDIR /src/www
RUN corepack enable && corepack prepare pnpm@9.15.9 --activate
COPY www/package.json www/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY www/ ./
RUN pnpm run build

# ── Backend ──────────────────────────────────────────────
FROM golang:1.24-alpine AS api
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/main .

# ── Runtime ──────────────────────────────────────────────
# Same layout as `make package`: the binary resolves www/ and data/
# relative to its own path, and reads i18n/ and config.json from its cwd.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata mariadb-client \
    && adduser -D -H -u 10001 urschema
WORKDIR /opt/urschema/app/bin
COPY --from=api /out/main ./main
COPY backend/config.json ./
COPY backend/i18n ./i18n
COPY --from=web /src/www/dist /opt/urschema/app/www
RUN mkdir -p /opt/urschema/data /opt/urschema/app/bin/logs \
    && chown -R urschema /opt/urschema/data /opt/urschema/app/bin/logs

# Inside a container the app must listen on all interfaces; the safe
# default is restored by publishing the port on 127.0.0.1 or by setting
# URSCHEMA_PASSWORD (see README "Security").
ENV HOST=0.0.0.0 PORT=8080
EXPOSE 8080
VOLUME /opt/urschema/data
USER urschema
ENTRYPOINT ["./main"]
