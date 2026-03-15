FROM golang:1.23-alpine AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o control-plane .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/control-plane .
EXPOSE 8080
CMD ["./control-plane"]
