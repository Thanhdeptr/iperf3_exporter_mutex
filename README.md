# iPerf3 Exporter v2 (Enhanced by ThanhDeptr)

A Prometheus exporter for iPerf3 network performance metrics with advanced mutex mechanism to prevent concurrent test conflicts.

---

> **This is a fork of [edgard/iperf3_exporter](https://github.com/edgard/iperf3_exporter) with additional enhancements by ThanhDeptr**

---

[![Go Report Card](https://goreportcard.com/badge/github.com/edgard/iperf3_exporter)](https://goreportcard.com/report/github.com/edgard/iperf3_exporter)
[![Docker Pulls](https://img.shields.io/docker/pulls/hatanthanh/iperf3_exporter.svg)](https://hub.docker.com/r/hatanthanh/iperf3_exporter)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/edgard/iperf3_exporter/blob/master/LICENSE)

## New Features (by ThanhDeptr)

- **Mutex Mechanism**: Prevents concurrent iPerf3 tests with global locking system
- **Bidirectional Testing**: Simultaneous upload/download bandwidth measurement
- **Bind Address Support**: Configure specific network interfaces for multi-WAN testing
- **Parallel Streams**: Enhanced performance with configurable parallel connections
- **Advanced Metrics**: Calculated bandwidth, latency, and packet loss metrics

---

## Mutex Mechanism

The enhanced exporter includes a global mutex system that prevents concurrent iPerf3 tests from conflicting with each other. This is especially useful when monitoring multiple WAN connections simultaneously:

- **Global Lock**: Only one iPerf3 test runs at a time across all requests
- **Queue Management**: Subsequent requests wait for the current test to complete
- **Timeout Protection**: Requests timeout after 80 seconds to prevent deadlocks
- **Lock Status Endpoint**: Monitor lock status via `/lock-status` endpoint

The iPerf3 exporter allows iPerf3 probing of endpoints for Prometheus monitoring, enabling you to measure network performance metrics like bandwidth, jitter, and packet loss.

## Features

- Measure network bandwidth between hosts
- Monitor network performance over time
- Support for both TCP and UDP tests
- Configurable test parameters (duration, bitrate, etc.)
- TLS support for secure communication
- Basic authentication for access control
- Health and readiness endpoints for monitoring
- Prometheus metrics for exporter itself

## Installation & Usage

### Using Docker (Recommended)

```bash
docker run -d \
  --name iperf3_exporter \
  -p 9579:9579 \
  --cap-add=NET_ADMIN \
  --cap-add=NET_RAW \
  hatanthanh/iperf3_exporter:latest
```

### From Binaries

```bash
# Download from releases
curl -L -o iperf3_exporter https://github.com/edgard/iperf3_exporter/releases/download/VERSION/iperf3_exporter-VERSION.PLATFORM
chmod +x iperf3_exporter
./iperf3_exporter --help
```

### Configuration

The exporter can be configured using command-line flags:

- `--web.listen-address`: Address to listen on (default: `:9579`)
- `--web.telemetry-path`: Path under which to expose metrics (default: `/metrics`)
- `--log.level`: Log level (default: `info`)

### Prometheus Configuration

Add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'iperf3'
    static_configs:
      - targets: ['localhost:9579']
    metrics_path: '/probe'
    params:
      target: ['your-target-host']
      port: ['5201']
      period: ['10s']
      bidirectional: ['true']
      bind_address: ['192.168.1.100']
      parallel: ['2']
```

## API Endpoints

- `/metrics`: Prometheus metrics
- `/probe`: iPerf3 probe endpoint
- `/health`: Health check endpoint
- `/ready`: Readiness check endpoint
- `/lock-status`: Mutex lock status (new feature)

## Parameters

- `target`: Target host for iPerf3 test
- `port`: Port number (default: 5201)
- `period`: Test duration (default: 10s)
- `bidirectional`: Enable bidirectional testing (default: false)
- `bind_address`: Bind to specific network interface
- `parallel`: Number of parallel streams (default: 1)

## Metrics

The exporter exposes the following metrics:

- `iperf3_up`: Whether the last iPerf3 probe was successful
- `iperf3_sent_bytes`: Total bytes sent
- `iperf3_received_bytes`: Total bytes received
- `iperf3_sent_seconds`: Total seconds spent sending
- `iperf3_received_seconds`: Total seconds spent receiving
- `iperf3_retransmits`: Total retransmits
- `iperf3_bandwidth_upload_mbps`: Upload bandwidth in Mbps
- `iperf3_bandwidth_download_mbps`: Download bandwidth in Mbps
- `iperf3_latency_upload_ms`: Upload latency in milliseconds
- `iperf3_latency_download_ms`: Download latency in milliseconds
- `iperf3_packet_loss_upload_percent`: Upload packet loss percentage
- `iperf3_packet_loss_download_percent`: Download packet loss percentage

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is released under Apache License 2.0, see [LICENSE](https://github.com/edgard/iperf3_exporter/blob/master/LICENSE).

**Original Repository**: [edgard/iperf3_exporter](https://github.com/edgard/iperf3_exporter)  
**Enhanced Fork v2**: [Thanhdeptr/iperf3_exporter_mutex](https://github.com/Thanhdeptr/iperf3_exporter_mutex)
