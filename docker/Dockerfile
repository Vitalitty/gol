# syntax=docker/dockerfile:1

FROM node:24-alpine AS frontend
WORKDIR /src/frontend

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

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates tzdata \
  && addgroup -S gol \
  && adduser -S -G gol gol \
  && mkdir -p /logs \
  && chown -R gol:gol /logs

COPY --from=builder /out/gol /usr/local/bin/gol

EXPOSE 3003
USER gol
ENTRYPOINT ["gol"]
CMD ["--host=0.0.0.0", "--open=false", "-f=/logs/*.log"]
