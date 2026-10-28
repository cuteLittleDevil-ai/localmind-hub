# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/hub ./apps/hub

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /out/hub /app/hub
COPY apps/hub/etc/hub-docker.yaml /app/etc/hub.yaml
RUN mkdir -p /app/data
EXPOSE 8888
CMD ["/app/hub", "-f", "etc/hub.yaml"]
