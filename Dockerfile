FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY peer/go.mod ./peer/go.mod
COPY peer/*.go ./peer/
COPY peer/account ./peer/account
COPY peer/dissycrypto ./peer/dissycrypto
COPY peer/helpers ./peer/helpers

WORKDIR /src/peer
RUN go build -o /out/peer .

FROM alpine:3.20

RUN addgroup -S pokoinpos && adduser -S -G pokoinpos pokoinpos
WORKDIR /app
COPY --from=builder /out/peer /app/peer
RUN mkdir -p /data && chown -R pokoinpos:pokoinpos /data

ENV POKOINPOS_RUN_MODE=node
ENV POKOINPOS_LISTEN_PORT=43000
ENV POKOINPOS_OPS_ADDR=:8080
ENV POKOINPOS_JOIN_HOST=127.0.0.1
ENV POKOINPOS_JOIN_PORT=-1
ENV POKOINPOS_SLOT_SECONDS=1
ENV POKOINPOS_GENESIS_HARDNESS=10000
ENV POKOINPOS_GENESIS_SEED=42
ENV POKOINPOS_INITIAL_BALANCE=1000000
ENV POKOINPOS_STATE_DIR=/data
ENV POKOINPOS_STATE_SAVE_INTERVAL_SECONDS=15
ENV POKOINPOS_RECONNECT_INTERVAL_SECONDS=5

EXPOSE 43000 8080
VOLUME ["/data"]
USER pokoinpos
ENTRYPOINT ["/app/peer"]
