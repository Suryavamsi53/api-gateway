FROM golang:1.23-alpine AS build
WORKDIR /src
COPY . .
RUN apk add --no-cache git
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway ./cmd/gateway

FROM scratch
COPY --from=build /gateway /gateway
EXPOSE 8080
ENTRYPOINT ["/gateway"]
