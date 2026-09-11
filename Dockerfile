# syntax=docker/dockerfile:1

FROM node:22-bookworm-slim AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.23-bookworm AS go-builder
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /src/web/dist/ internal/webui/dist/
RUN CGO_ENABLED=1 go build -trimpath \
    -ldflags "-s -w -X netprob/internal/buildinfo.Version=${VERSION} -X netprob/internal/buildinfo.Commit=${COMMIT} -X netprob/internal/buildinfo.Date=${BUILD_DATE}" \
    -o /out/netprob ./cmd/netprob

FROM debian:bookworm-slim
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
LABEL org.opencontainers.image.title="NetProb" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates iputils-ping mtr-tiny \
    && rm -rf /var/lib/apt/lists/*
COPY --from=go-builder /out/netprob /usr/local/bin/netprob
RUN mkdir -p /var/lib/netprob
ENV NETPROB_SERVER_LISTEN=:8080 \
    NETPROB_DATA_DIR=/var/lib/netprob
VOLUME ["/var/lib/netprob"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/netprob"]
CMD ["-mode", "server"]
