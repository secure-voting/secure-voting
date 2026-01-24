# ---- building ----

FROM rust:1.92 AS builder

WORKDIR /app
COPY ./ ./
RUN cargo build --release

# ---- runtime ----

FROM debian:bookworm-slim
WORKDIR /app

COPY --from=builder /app/target/release/compute-service .

EXPOSE 50051
CMD ["./compute-service"]
