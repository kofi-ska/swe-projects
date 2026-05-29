use std::net::SocketAddr;

use solana_compute::proto::solana::v1::compute_service_server::ComputeServiceServer;
use solana_compute::service::ComputeServiceImpl;
use tokio::signal;
use tonic::transport::Server;
use tracing_subscriber::EnvFilter;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::from_default_env())
        .compact()
        .init();

    let addr: SocketAddr = std::env::var("COMPUTE_ADDR")
        .unwrap_or_else(|_| "0.0.0.0:50051".to_string())
        .parse()?;

    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .worker_threads(worker_threads())
        .build()?;

    runtime.block_on(async move {
        let service = ComputeServiceImpl::default();
        let server = Server::builder()
            .add_service(ComputeServiceServer::new(service))
            .serve_with_shutdown(addr, async {
                let _ = signal::ctrl_c().await;
            });

        tracing::info!("compute listening on {}", addr);
        server.await
    })?;

    Ok(())
}

fn worker_threads() -> usize {
    std::thread::available_parallelism()
        .map(|count| count.get())
        .unwrap_or(4)
}
