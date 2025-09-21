FROM golang:1.25-alpine AS builder
COPY ./ /core
WORKDIR /core
RUN sh build.sh



FROM alpine:3.20 AS runtime
COPY --from=builder /core/proxy-pool /usr/bin/proxy-pool
WORKDIR /work
ENTRYPOINT ["/usr/bin/proxy-pool"]


