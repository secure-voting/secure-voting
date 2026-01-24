fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_prost_build::configure()
        .build_client(false)
        .compile_protos(&["../proto/compute_v1.proto"], &["../proto"])?;
    Ok(())
}
