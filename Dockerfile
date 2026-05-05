FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o db-gateway .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/db-gateway /usr/local/bin/db-gateway
EXPOSE 5432
ENTRYPOINT ["db-gateway"]
