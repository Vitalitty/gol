# syntax=docker/dockerfile:1

FROM node:24-alpine AS frontend
ENV ASTRO_TELEMETRY_DISABLED=1
WORKDIR /src/frontend

RUN npm install -g npm@11.16.0

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM golang:1.26.3-alpine AS builder
ARG VERSION=container
WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
COPY --from=frontend /src/frontend/dist ./frontend/dist

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
  -ldflags="-s -w -X main.version=${VERSION}" \
  -o /out/gol ./frontend

FROM alpine:3.23 AS runtime
RUN apk add --no-cache bash ca-certificates tzdata \
  && addgroup -S gol \
  && adduser -S -G gol gol \
  && mkdir -p /logs \
  && chown -R gol:gol /logs

COPY --from=builder /out/gol /usr/local/bin/gol
COPY docker/gol-entrypoint.sh /usr/local/bin/gol-entrypoint
RUN chmod +x /usr/local/bin/gol-entrypoint

EXPOSE 3003
USER gol
ENTRYPOINT ["gol-entrypoint"]
