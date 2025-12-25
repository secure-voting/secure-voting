FROM rust:1.92

WORKDIR /app

COPY Cargo.toml .

RUN cargo build --release

COPY target/release/* .

EXPOSE 8081

CMD ["./main"]
