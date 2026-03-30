FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -trimpath -o lazy-tpc-proxy .

FROM scratch
COPY --from=builder /app/lazy-tpc-proxy /lazy-tpc-proxy
ENV DOCKER_SOCK=/var/run/docker.sock
VOLUME ["/var/run/docker.sock"]
ENTRYPOINT ["/lazy-tpc-proxy"]
