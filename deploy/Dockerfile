FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/pds ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 pds
COPY --from=build /out/pds /usr/local/bin/pds
USER 10001
EXPOSE 8080 9090 8081
ENTRYPOINT ["/usr/local/bin/pds"]

