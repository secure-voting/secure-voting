[![unsafe forbidden](https://img.shields.io/badge/unsafe-forbidden-success.svg)](https://github.com/rust-secure-code/safety-dance/)
[![MSRV](https://img.shields.io/badge/MSRV-1.92.0-blue)](https://github.com/schukark/secure-voting)
![CI](https://github.com/schukark/secure-voting/actions/workflows/rust-ci.yml/badge.svg)
![licenses](https://img.shields.io/badge/licenses-MIT%2FApache--2.0-blue)

# Compute service

`compute-service` provides a gRPC server implementation to process election calculations.

The proto spec is defined at `../../backend/proto/compute_v1.proto`.
Specification is shared between the backend, which acts as the client and the Rust compute service, which acts as a server.
