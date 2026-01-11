FROM golang:1.21-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/server ./cmd/server

FROM alpine:3.19

RUN addgroup -S app && adduser -S app -G app
USER app
WORKDIR /app

COPY --from=build /out/server /app/server
COPY public /app/public

EXPOSE 8082
ENTRYPOINT ["/app/server"]
