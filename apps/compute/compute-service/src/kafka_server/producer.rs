use rdkafka::{ClientConfig, producer::FutureProducer};

fn setup_producer(brokers: &str, timeout: usize) -> FutureProducer {
    ClientConfig::new()
        .set("bootstrap.servers", brokers)
        .set("message.timeout.ms", timeout.to_string())
        .create()
        .expect("Failed to create a producer.")
}
