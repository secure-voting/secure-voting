# ---- building ----

FROM rust:1.92 AS builder


RUN apt-get update \
  && apt-get install -y --no-install-recommends protobuf-compiler libprotobuf-dev ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY ./ ./
RUN cargo build --release --bin compute-service

# ---- runtime ----

FROM debian:bookworm-slim
WORKDIR /app

COPY --from=builder /app/target/release/compute-service .

EXPOSE 50051
CMD ["./compute-service"]
