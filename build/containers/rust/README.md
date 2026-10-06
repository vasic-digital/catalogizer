# IMG-RUST

Rust and Tauri build image: rustup-init 1.29.0 (SHA-256 verified), toolchain 1.99.0, `cargo-llvm-cov` 0.9.1 (the Rust coverage tool chosen here, with the `llvm-tools-preview` component; `cargo tarpaulin` is not used), `tauri-cli` 2.12.1, the webkit2gtk, appindicator, rsvg and gtk development packages, xvfb. Replaces the `curl | sh` rustup and the unpinned `cargo install tauri-cli` of `docker/Dockerfile.builder` (D-05).
The Containerfile is authored by T106; its first remote build and its `images.lock.yaml` entry are owned by T143 (WP-14): this task wrote no entry and built no image. Existence verdict of the coverage tool: `evidence/wp11/rust-coverage-tools.json`. Not built, so nothing in it is verified beyond static checks.
