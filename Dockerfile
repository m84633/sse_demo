FROM golang:1.21-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server

FROM alpine:3.19

RUN addgroup -S app && adduser -S app -G app
USER app
WORKDIR /app

COPY --from=build /out/server /app/server

EXPOSE 8082
ENTRYPOINT ["/app/server"]
