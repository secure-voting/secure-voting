use opentelemetry::{
    global,
    metrics::{Counter, Histogram, UpDownCounter},
};

#[derive(Debug)]
pub struct ComputeMetrics {
    pub(crate) requests: Counter<u64>,
    pub(crate) failures: Counter<u64>,
    pub(crate) duration: Histogram<f64>,
    pub(crate) active_requests: UpDownCounter<i64>,
}

impl ComputeMetrics {
    #[must_use]
    pub fn init() -> ComputeMetrics {
        let meter = global::meter("rust-compute");

        ComputeMetrics {
            requests: meter.u64_counter("compute.requests.total").build(),
            failures: meter.u64_counter("compute.failures.total").build(),
            duration: meter
                .f64_histogram("compute.duration.seconds")
                .with_unit("s")
                .build(),
            active_requests: meter.i64_up_down_counter("compute.active_requests").build(),
        }
    }
}

impl Default for ComputeMetrics {
    fn default() -> Self {
        Self::init()
    }
}
