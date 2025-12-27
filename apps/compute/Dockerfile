FROM rust:1.92

WORKDIR /app

COPY ./ ./

RUN cargo build --release

EXPOSE 8081

CMD ["./target/release/compute-service"]
