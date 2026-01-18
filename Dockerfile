FROM golang:1.23 AS build
WORKDIR /src

# Copy go.mod and go.sum first for better docker cache
COPY go.mod go.sum ./

# Install git
RUN apt-get update && apt-get install -y git ca-certificates && rm -rf /var/lib/apt/lists/*

# Download dependencies
RUN go mod download

# Copy the rest of the source
COPY . .

# Build the gateway binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /gateway ./cmd/gateway

# Use distroless image for smaller final image
FROM gcr.io/distroless/static:nonroot
COPY --from=build /gateway /gateway
EXPOSE 8080
ENTRYPOINT ["/gateway"]
