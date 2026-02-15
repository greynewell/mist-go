# Build stage: compile a static binary with zero CGO dependencies.
FROM golang:1.24-alpine AS build
RUN apk add --no-cache ca-certificates
WORKDIR /src
COPY go.mod ./
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /mist ./cmd/mist

# Runtime: scratch container — just the binary and CA certs.
# Final image is typically < 10MB.
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /mist /mist
ENTRYPOINT ["/mist"]
