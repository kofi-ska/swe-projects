fn main() {
    tonic_build::configure()
        .build_client(true)
        .build_server(true)
        .compile(&["../../../contracts/decision.proto"], &["../../../contracts"])
        .expect("failed to compile protos");
}
