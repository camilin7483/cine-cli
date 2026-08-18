# multi-stage static-ish build
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/cine ./cmd/cine

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates mpv yt-dlp \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/cine /usr/local/bin/cine
ENTRYPOINT ["cine"]
