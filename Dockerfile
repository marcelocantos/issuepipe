# Ubuntu 24.04 ships g++ ≥13 with C++23 <format>, required by sqlpipe.
FROM ubuntu:24.04 AS build
WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl g++ make \
 && rm -rf /var/lib/apt/lists/* \
 && curl -fsSL https://go.dev/dl/go1.25.7.linux-amd64.tar.gz \
    | tar -C /usr/local -xz
ENV PATH=/usr/local/go/bin:$PATH

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /issuepipe ./cmd/issuepipe

# Runtime needs libstdc++ for the CGO-linked sqlpipe binary.
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libstdc++6 \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir -p /data
COPY --from=build /issuepipe /issuepipe
EXPOSE 8080
ENV LISTEN_ADDR=:8080 DATA_DIR=/data
CMD ["/issuepipe"]
