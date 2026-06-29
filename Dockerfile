FROM golang:1.25.0-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o digestBot ./cmd/digestBot


FROM alpine:3.23
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY prompt.txt ./
COPY --from=backend-builder /app/digestBot .
CMD ["./digestBot"]