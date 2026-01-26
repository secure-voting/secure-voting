use rdkafka::{
    ClientConfig,
    consumer::{Consumer, StreamConsumer},
};

fn setup_consumer(brokers: &str, timeout: usize, group_id: &str, topic: &str) -> StreamConsumer {
    let consumer: StreamConsumer = ClientConfig::new()
        .set("bootstrap.servers", brokers)
        .set("session.timeout.ms", timeout.to_string())
        .set("enable.auto.commit", "false")
        .set("group.id", group_id)
        .create()
        .expect("Failed to create a consumer.");
    consumer
        .subscribe(&[topic])
        .expect("Failed to subscribe to a topic");

    consumer
}
