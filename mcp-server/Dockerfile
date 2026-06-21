# Build the Claworc MCP server as a small static binary.
FROM golang:1.25-alpine AS build

WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/claworc-mcp .

# Minimal runtime: static binary + CA certificates for HTTPS control planes.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/claworc-mcp /usr/local/bin/claworc-mcp

# The server communicates over stdio; run with `docker run -i`.
ENTRYPOINT ["/usr/local/bin/claworc-mcp"]
