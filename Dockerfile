FROM golang:1.25-bookworm AS build
WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    g++ make ca-certificates \
 && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /issuepipe ./cmd/issuepipe

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir -p /data
COPY --from=build /issuepipe /issuepipe
EXPOSE 8080
ENV LISTEN_ADDR=:8080 DATA_DIR=/data
CMD ["/issuepipe"]
