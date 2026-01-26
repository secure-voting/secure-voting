use futures::future::join_all;

use crate::rpc_server::run_server;

mod kafka_server;
mod rpc_server;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let addr: std::net::SocketAddr = std::env::var("GRPC_ADDR")
        .unwrap_or_else(|_| "0.0.0.0:50051".to_string())
        .parse()?;

    let rpc_server_join_handle = tokio::spawn(async move { run_server(addr).await });

    join_all([rpc_server_join_handle]).await;
    Ok(())
}
