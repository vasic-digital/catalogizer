# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0
#
# OCU CUDA sidecar — GPU workload server for thinker.local.
#
# Build (on a CUDA-capable host or cross-compile with buildx):
#   podman build --network host -t ocu-cuda-sidecar:latest .
#
# Run on thinker.local (requires NVIDIA container runtime):
#   podman run --device nvidia.com/gpu=all --rm -p 50060:50060 ocu-cuda-sidecar:latest

# ---------------------------------------------------------------------------
# Stage 1: builder
# ---------------------------------------------------------------------------
FROM nvidia/cuda:12.2.0-devel-ubuntu22.04 AS builder

ARG GO_VERSION=1.25.3
ARG TARGETARCH=amd64

ENV DEBIAN_FRONTEND=noninteractive
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=1

RUN apt-get update && apt-get install -y --no-install-recommends \
    wget \
    ca-certificates \
    build-essential \
    pkg-config \
    cmake \
    git \
    libopencv-dev \
    libopencv-contrib-dev \
    ffmpeg \
    tesseract-ocr \
    libtesseract-dev \
    && rm -rf /var/lib/apt/lists/*

# Install Go.
RUN wget -q "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" \
 && tar -C /usr/local -xzf "go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" \
 && rm "go${GO_VERSION}.linux-${TARGETARCH}.tar.gz"

ENV PATH=/usr/local/go/bin:${PATH}
ENV GOPATH=/go

# gocv requires OpenCV headers; point pkg-config at the system install.
ENV PKG_CONFIG_PATH=/usr/lib/x86_64-linux-gnu/pkgconfig

WORKDIR /src
COPY go.mod go.sum ./
RUN GOTOOLCHAIN=local go mod download

COPY . .

# Build with the cuda tag so the GPU backend is compiled in.
RUN GOTOOLCHAIN=local go build -tags=cuda \
    -ldflags="-s -w" \
    -o /out/ocu-cuda-sidecar \
    ./cmd/ocu-cuda-sidecar

# ---------------------------------------------------------------------------
# Stage 2: runtime image
# ---------------------------------------------------------------------------
FROM nvidia/cuda:12.2.0-runtime-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    libopencv-core4.5d \
    libopencv-imgproc4.5d \
    libopencv-videoio4.5d \
    libopencv-features2d4.5d \
    ffmpeg \
    tesseract-ocr \
    libtesseract4 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/ocu-cuda-sidecar /usr/local/bin/ocu-cuda-sidecar

# Non-root runtime user (GPU access comes from the container runtime's device injection, not from root)
RUN useradd --system --uid 10001 --no-create-home --shell /usr/sbin/nologin ocu

# Default listen address.
ENV OCU_LISTEN=0.0.0.0:50060
EXPOSE 50060

# Health check: the gRPC listener accepts TCP connections (bash is present in the CUDA ubuntu base)
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD bash -c 'exec 3<>/dev/tcp/127.0.0.1/50060' || exit 1

USER ocu

ENTRYPOINT ["/usr/local/bin/ocu-cuda-sidecar"]
CMD ["--listen", "0.0.0.0:50060"]
