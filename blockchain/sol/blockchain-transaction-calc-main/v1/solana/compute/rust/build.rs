fn main() {
    tonic_build::configure()
        .build_client(true)
        .build_server(true)
        .compile_protos(&["../../../contracts/decision.proto"], &["../../../contracts"])
        .expect("failed to compile protos");
}
