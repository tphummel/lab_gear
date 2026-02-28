FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy module definitions and download dependencies.
# GONOSUMDB=* avoids checksum database lookups for internal/air-gapped builds.
COPY go.mod go.sum ./
RUN GONOSUMDB="*" go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -ldflags="-s -w" -o lab_gear ./cmd/server

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/lab_gear .

EXPOSE 8080

CMD ["./lab_gear"]
