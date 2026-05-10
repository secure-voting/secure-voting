//! System metrics module.
//!
//! Contains the implementation of the struct `SystemMetrics`,
//! containing various system metrics like cpu and ram usage.

use serde::Serialize;
use std::time::Instant;
use sysinfo::{Pid, ProcessRefreshKind, ProcessesToUpdate, RefreshKind, System};
use tracing::info;

/// Metrics result containing timing, memory, and CPU measurements.
#[derive(Debug, Clone, Serialize)]
pub struct SystemMetrics {
    /// Total execution time in milliseconds.
    pub total_ms: u64,
    /// Resident set size (memory used by process) in bytes.
    pub memory_rss_bytes: u64,
    /// CPU usage percentage (can be >100 on multi-core).
    pub cpu_usage_percent: f32,
    /// Number of ballots processed.
    pub ballots_count: usize,
    /// Throughput: ballots processed per second.
    pub throughput_ballots_per_sec: f64,
}

/// Collector for system metrics, to be created per request.
pub struct SystemMetricsCollector {
    /// sysinfo's System, responsible for taking measurements of various system metrics.
    system: System,
    /// Initial memory usage before algorithm execution.
    memory_start: u64,
    /// Initial cpu usage before algorithm execution.
    cpu_start: f32,
    /// Starting time of the execution.
    start_time: Instant,
    /// Ballot count of the profile.
    ballots_count: usize,
}

impl SystemMetricsCollector {
    /// Create a new collector, capturing initial state.
    ///
    /// Should be called at the start of a request.
    ///
    /// # Panics
    ///
    /// Panics if the process cannot be found.
    #[allow(clippy::expect_used)]
    #[must_use]
    pub fn new(ballots_count: usize) -> Self {
        let mut system = System::new_with_specifics(
            RefreshKind::nothing().with_processes(ProcessRefreshKind::everything()),
        );
        system.refresh_processes_specifics(
            ProcessesToUpdate::All,
            true,
            ProcessRefreshKind::everything(),
        );

        let pid = Pid::from_u32(std::process::id());
        let process = system.process(pid).expect("process not found");
        let memory_start = process.memory();
        let cpu_start = process.cpu_usage();

        std::thread::sleep(sysinfo::MINIMUM_CPU_UPDATE_INTERVAL);

        SystemMetricsCollector {
            system,
            memory_start,
            cpu_start,
            start_time: Instant::now(),
            ballots_count,
        }
    }

    /// Measure the state difference between current and initial state.
    ///
    /// Should be called at the end of a request.
    ///
    /// # Panics
    ///
    /// Panics if the process cannot be found.
    #[allow(clippy::expect_used)]
    #[allow(clippy::cast_precision_loss)]
    #[allow(clippy::cast_possible_truncation)]
    #[must_use]
    pub fn measure(&mut self) -> SystemMetrics {
        self.system.refresh_processes_specifics(
            ProcessesToUpdate::All,
            true,
            ProcessRefreshKind::everything(),
        );

        let pid = Pid::from_u32(std::process::id());
        let process = self.system.process(pid).expect("process not found");

        let memory_end = process.memory();
        let cpu_end = process.cpu_usage();
        let elapsed_ms = self.start_time.elapsed().as_millis() as u64;
        let throughput = if elapsed_ms > 0 {
            (self.ballots_count as f64) / (elapsed_ms as f64 / 1000.0)
        } else {
            0.0
        };

        info!(
            total_ms = elapsed_ms,
            memory_delta_bytes = memory_end.saturating_sub(self.memory_start),
            cpu_percent = cpu_end - self.cpu_start,
            ballots = self.ballots_count,
            throughput_per_sec = throughput,
            "System metrics collected"
        );

        SystemMetrics {
            total_ms: elapsed_ms,
            memory_rss_bytes: memory_end.saturating_sub(self.memory_start),
            cpu_usage_percent: cpu_end - self.cpu_start,
            ballots_count: self.ballots_count,
            throughput_ballots_per_sec: throughput,
        }
    }
}
