FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -trimpath -o lazy-tcp-proxy .

FROM scratch
COPY --from=builder /app/lazy-tcp-proxy /lazy-tcp-proxy
COPY ./LICENSE /LICENSE
ENV DOCKER_SOCK=/var/run/docker.sock
VOLUME ["/var/run/docker.sock"]
ENTRYPOINT ["/lazy-tcp-proxy"]
