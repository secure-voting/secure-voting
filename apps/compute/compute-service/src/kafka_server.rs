use rdkafka::{
    ClientConfig,
    consumer::{Consumer, StreamConsumer},
    error::KafkaResult,
    producer::FutureProducer,
};

fn setup_producer(brokers: &str, timeout: usize) -> KafkaResult<FutureProducer> {
    ClientConfig::new()
        .set("bootstrap.servers", brokers)
        .set("message.timeout.ms", timeout.to_string())
        .create()
}

fn setup_consumer(
    brokers: &str,
    timeout: usize,
    group_id: &str,
    topic: &str,
) -> KafkaResult<StreamConsumer> {
    let consumer: StreamConsumer = ClientConfig::new()
        .set("bootstrap.servers", brokers)
        .set("session.timeout.ms", timeout.to_string())
        .set("enable.auto.commit", "false")
        .set("group.id", group_id)
        .create()?;
    consumer.subscribe(&[topic])?;

    Ok(consumer)
}
