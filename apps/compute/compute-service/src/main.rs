use crate::securevoting::compute::v1::compute_server::{Compute, ComputeServer};

pub mod securevoting {
    pub mod compute {
        pub mod v1 {
            tonic::include_proto!("securevoting.compute.v1");
        }
    }
}

use securevoting::compute::v1::{RunChunk, RunResult};

#[derive(Debug, Default)]
struct ComputeService;

#[tonic::async_trait]
impl Compute for ComputeService {
    async fn run(
        &self,
        request: tonic::Request<tonic::Streaming<RunChunk>>,
    ) -> Result<tonic::Response<RunResult>, tonic::Status> {
        todo!()
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr = "[::1]:50051".parse()?;
    let greeter = ComputeService;

    tonic::transport::Server::builder()
        .add_service(ComputeServer::new(greeter))
        .serve(addr)
        .await?;

    Ok(())
}
