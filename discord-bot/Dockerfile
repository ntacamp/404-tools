FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bot ./cmd/bot

# distroless/static ships CA certificates (Discord, schedule URL) and tzdata
# (config.yaml's timezone).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bot /bot
COPY config.yaml /config.yaml
# DISCORD_BOT_TOKEN must be provided at runtime.
ENTRYPOINT ["/bot", "-config", "/config.yaml"]
