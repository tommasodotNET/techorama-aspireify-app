FROM docker.io/library/golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -o /catbooks .

FROM docker.io/library/alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 catbooks
COPY --from=build /catbooks /usr/local/bin/catbooks
USER catbooks
EXPOSE 8080
CMD ["catbooks"]
