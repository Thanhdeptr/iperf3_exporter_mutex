// Copyright 2019 Edgard Castro
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package collector provides the Prometheus collector for iperf3 metrics.
package collector

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/edgard/iperf3_exporter/internal/iperf"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "iperf3"
)

// Metrics about the iperf3 exporter itself.
var (
	IperfDuration = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Name: prometheus.BuildFQName(namespace, "exporter", "duration_seconds"),
			Help: "Duration of collections by the iperf3 exporter.",
		},
	)
	IperfErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, "exporter", "errors_total"),
			Help: "Errors raised by the iperf3 exporter.",
		},
	)
)

// ProbeConfig represents the configuration for a single probe.
type ProbeConfig struct {
	Target        string
	Port          int
	Period        time.Duration
	Timeout       time.Duration
	ReverseMode   bool
	Bidirectional bool // Enhanced feature by ThanhDeptr: run both upload and download tests
	UDPMode       bool
	Bitrate       string
	BindAddress   string          // Source IP address to bind to (-B parameter)
	Parallel      int             // Number of parallel streams (-P parameter)
	Context       context.Context // Request context for proper cancellation
}

// Collector implements the prometheus.Collector interface for iperf3 metrics.
type Collector struct {
	target        string
	port          int
	period        time.Duration
	timeout       time.Duration
	mutex         sync.RWMutex
	reverse       bool
	bidirectional bool // Enhanced feature by ThanhDeptr
	udpMode       bool
	bitrate       string
	bindAddress   string
	parallel      int
	context       context.Context // Request context for proper cancellation
	logger        *slog.Logger
	runner        iperf.Runner

	// Metrics
	up              *prometheus.Desc
	sentSeconds     *prometheus.Desc
	sentBytes       *prometheus.Desc
	receivedSeconds *prometheus.Desc
	receivedBytes   *prometheus.Desc
	// TCP-specific metrics
	retransmits *prometheus.Desc
	// UDP-specific metrics
	sentPackets     *prometheus.Desc
	sentJitter      *prometheus.Desc
	sentLostPackets *prometheus.Desc
	sentLostPercent *prometheus.Desc
	recvPackets     *prometheus.Desc
	recvJitter      *prometheus.Desc
	recvLostPackets *prometheus.Desc
	recvLostPercent *prometheus.Desc
	// Calculated metrics (Enhanced by ThanhDeptr)
	bandwidthUploadMbps   *prometheus.Desc
	bandwidthDownloadMbps *prometheus.Desc
	packetLossUpload      *prometheus.Desc
	packetLossDownload    *prometheus.Desc
	latencyUpload         *prometheus.Desc
	latencyDownload       *prometheus.Desc
}

// NewCollector creates a new Collector for iperf3 metrics.
func NewCollector(config ProbeConfig, logger *slog.Logger) *Collector {
	return NewCollectorWithRunner(config, logger, iperf.NewRunner(logger))
}

// NewCollectorWithRunner creates a new Collector for iperf3 metrics with a custom runner.
func NewCollectorWithRunner(config ProbeConfig, logger *slog.Logger, runner iperf.Runner) *Collector {
	// Common labels for all metrics
	labels := []string{"target", "port", "direction"}

	return &Collector{
		target:        config.Target,
		port:          config.Port,
		period:        config.Period,
		timeout:       config.Timeout,
		reverse:       config.ReverseMode,
		bidirectional: config.Bidirectional,
		udpMode:       config.UDPMode,
		bitrate:       config.Bitrate,
		bindAddress:   config.BindAddress,
		parallel:      config.Parallel,
		context:       config.Context,
		logger:        logger,
		runner:        runner,

		// Define metrics with labels
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Was the last iperf3 probe successful (1 for success, 0 for failure).",
			labels, nil,
		),
		sentSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_seconds"),
			"Total seconds spent sending packets.",
			labels, nil,
		),
		sentBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_bytes"),
			"Total sent bytes for the last test run.",
			labels, nil,
		),
		receivedSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_seconds"),
			"Total seconds spent receiving packets.",
			labels, nil,
		),
		receivedBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_bytes"),
			"Total received bytes for the last test run.",
			labels, nil,
		),
		// TCP-specific metrics
		retransmits: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "retransmits"),
			"Total retransmits for the last test run.",
			labels, nil,
		),
		// UDP-specific metrics
		sentPackets: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_packets"),
			"Total sent packets for the last UDP test run.",
			labels, nil,
		),
		sentJitter: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_jitter_ms"),
			"Jitter in milliseconds for sent packets in UDP mode.",
			labels, nil,
		),
		sentLostPackets: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_lost_packets"),
			"Total lost packets from the sender in the last UDP test run.",
			labels, nil,
		),
		sentLostPercent: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_lost_percent"),
			"Percentage of packets lost from the sender in the last UDP test run.",
			labels, nil,
		),
		recvPackets: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_packets"),
			"Total received packets for the last UDP test run.",
			labels, nil,
		),
		recvJitter: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_jitter_ms"),
			"Jitter in milliseconds for received packets in UDP mode.",
			labels, nil,
		),
		recvLostPackets: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_lost_packets"),
			"Total lost packets at the receiver in the last UDP test run.",
			labels, nil,
		),
		recvLostPercent: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "received_lost_percent"),
			"Percentage of packets lost at the receiver in the last UDP test run.",
			labels, nil,
		),
		// Calculated metrics (Enhanced by ThanhDeptr)
		bandwidthUploadMbps: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "bandwidth_upload_mbps"),
			"Upload bandwidth in Mbps (calculated from sent bytes and time).",
			labels, nil,
		),
		bandwidthDownloadMbps: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "bandwidth_download_mbps"),
			"Download bandwidth in Mbps (calculated from received bytes and time).",
			labels, nil,
		),
		packetLossUpload: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "packet_loss_upload_percent"),
			"Upload packet loss percentage (calculated from sent vs received bytes).",
			labels, nil,
		),
		packetLossDownload: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "packet_loss_download_percent"),
			"Download packet loss percentage (calculated from sent vs received bytes).",
			labels, nil,
		),
		latencyUpload: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "latency_upload_ms"),
			"Upload latency in milliseconds (calculated from retransmits and time).",
			labels, nil,
		),
		latencyDownload: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "latency_download_ms"),
			"Download latency in milliseconds (calculated from retransmits and time).",
			labels, nil,
		),
	}
}

// Describe implements the prometheus.Collector interface.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.sentSeconds
	ch <- c.sentBytes
	ch <- c.receivedSeconds
	ch <- c.receivedBytes

	// TCP-specific metrics
	ch <- c.retransmits

	// UDP-specific metrics
	ch <- c.sentPackets
	ch <- c.sentJitter
	ch <- c.sentLostPackets
	ch <- c.sentLostPercent
	ch <- c.recvPackets
	ch <- c.recvJitter
	ch <- c.recvLostPackets
	ch <- c.recvLostPercent

	// Calculated metrics (Enhanced by ThanhDeptr)
	ch <- c.bandwidthUploadMbps
	ch <- c.bandwidthDownloadMbps
	ch <- c.packetLossUpload
	ch <- c.packetLossDownload
	ch <- c.latencyUpload
	ch <- c.latencyDownload
}

// Collect implements the prometheus.Collector interface.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock() // To protect metrics from concurrent collects.
	defer c.mutex.Unlock()

	// Use request context if available, otherwise create one with timeout
	var ctx context.Context
	var cancel context.CancelFunc
	if c.context != nil {
		ctx, cancel = context.WithTimeout(c.context, c.timeout)
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), c.timeout)
	}
	defer cancel()

	// Common label values for all metrics
	labelValues := []string{c.target, strconv.Itoa(c.port)}

	// Check if bidirectional mode is enabled (enhanced feature by ThanhDeptr)
	if c.bidirectional {
		// Run both upload and download tests sequentially to avoid conflicts
		c.logger.Debug("Running bidirectional test sequentially", "target", c.target, "port", c.port)

		// Create separate context for each test to avoid timeout issues
		uploadCtx, uploadCancel := context.WithTimeout(c.context, c.timeout)
		defer uploadCancel()

		// Run upload test (no -R flag)
		c.logger.Debug("Starting upload test", "target", c.target, "bind_address", c.bindAddress)
		uploadResult := c.runner.Run(uploadCtx, iperf.Config{
			Target:      c.target,
			Port:        c.port,
			Period:      c.period,
			Timeout:     c.timeout,
			ReverseMode: false, // Upload test
			UDPMode:     c.udpMode,
			Bitrate:     c.bitrate,
			BindAddress: c.bindAddress,
			Parallel:    c.parallel,
			Logger:      c.logger,
		})

		// Small delay between tests to avoid resource conflicts
		// Increased to 35s to allow iPerf3 server to fully reset
		time.Sleep(35 * time.Second)

		// Create separate context for download test
		downloadCtx, downloadCancel := context.WithTimeout(c.context, c.timeout)
		defer downloadCancel()

		// Run download test (with -R flag)
		c.logger.Debug("Starting download test", "target", c.target, "bind_address", c.bindAddress)
		downloadResult := c.runner.Run(downloadCtx, iperf.Config{
			Target:      c.target,
			Port:        c.port,
			Period:      c.period,
			Timeout:     c.timeout,
			ReverseMode: true, // Download test
			UDPMode:     c.udpMode,
			Bitrate:     c.bitrate,
			BindAddress: c.bindAddress,
			Parallel:    c.parallel,
			Logger:      c.logger,
		})

		// Process upload results
		uploadLabels := append(labelValues, "upload")
		c.processResult(ch, uploadResult, uploadLabels)

		// Process download results
		downloadLabels := append(labelValues, "download")
		c.processResult(ch, downloadResult, downloadLabels)

		return
	}

	// Original single-direction test
	result := c.runner.Run(ctx, iperf.Config{
		Target:      c.target,
		Port:        c.port,
		Period:      c.period,
		Timeout:     c.timeout,
		ReverseMode: c.reverse,
		UDPMode:     c.udpMode,
		Bitrate:     c.bitrate,
		BindAddress: c.bindAddress,
		Parallel:    c.parallel,
		Logger:      c.logger,
	})

	// Process single-direction result
	direction := "upload"
	if c.reverse {
		direction = "download"
	}
	singleLabels := append(labelValues, direction)
	c.processResult(ch, result, singleLabels)
}

// processResult is a helper function to process iperf3 test results and emit metrics
func (c *Collector) processResult(ch chan<- prometheus.Metric, result iperf.Result, labelValues []string) {
	// Set metrics based on result
	if result.Success {
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.sentSeconds, prometheus.GaugeValue, result.SentSeconds, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.sentBytes, prometheus.GaugeValue, result.SentBytes, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.receivedSeconds, prometheus.GaugeValue, result.ReceivedSeconds, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.receivedBytes, prometheus.GaugeValue, result.ReceivedBytes, labelValues...)

		// Retransmits is only relevant in TCP mode
		if !result.UDPMode {
			ch <- prometheus.MustNewConstMetric(c.retransmits, prometheus.GaugeValue, result.Retransmits, labelValues...)
		}

		// Include UDP-specific metrics when in UDP mode
		if result.UDPMode {
			ch <- prometheus.MustNewConstMetric(c.sentPackets, prometheus.GaugeValue, result.SentPackets, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentJitter, prometheus.GaugeValue, result.SentJitter, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentLostPackets, prometheus.GaugeValue, result.SentLostPackets, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentLostPercent, prometheus.GaugeValue, result.SentLostPercent, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvPackets, prometheus.GaugeValue, result.ReceivedPackets, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvJitter, prometheus.GaugeValue, result.ReceivedJitter, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvLostPackets, prometheus.GaugeValue, result.ReceivedLostPackets, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvLostPercent, prometheus.GaugeValue, result.ReceivedLostPercent, labelValues...)
		}

		// Calculate and emit computed metrics (Enhanced by ThanhDeptr)
		c.emitCalculatedMetrics(ch, result, labelValues)
	} else {
		// Return common metrics with 0 values when iperf3 fails
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.sentSeconds, prometheus.GaugeValue, 0, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.sentBytes, prometheus.GaugeValue, 0, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.receivedSeconds, prometheus.GaugeValue, 0, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.receivedBytes, prometheus.GaugeValue, 0, labelValues...)

		// Only include mode-specific metrics for the active mode
		if !result.UDPMode {
			// TCP-specific metrics on failure
			ch <- prometheus.MustNewConstMetric(c.retransmits, prometheus.GaugeValue, 0, labelValues...)
		} else {
			// UDP-specific metrics on failure
			ch <- prometheus.MustNewConstMetric(c.sentPackets, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentJitter, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentLostPackets, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.sentLostPercent, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvPackets, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvJitter, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvLostPackets, prometheus.GaugeValue, 0, labelValues...)
			ch <- prometheus.MustNewConstMetric(c.recvLostPercent, prometheus.GaugeValue, 0, labelValues...)
		}

		// Emit zero values for calculated metrics on failure
		c.emitCalculatedMetrics(ch, result, labelValues)
		IperfErrors.Inc()
	}
}

// emitCalculatedMetrics calculates and emits computed metrics (Enhanced by ThanhDeptr)
func (c *Collector) emitCalculatedMetrics(ch chan<- prometheus.Metric, result iperf.Result, labelValues []string) {
	// Calculate bandwidth in Mbps
	var uploadMbps, downloadMbps float64
	if result.SentSeconds > 0 {
		uploadMbps = (result.SentBytes * 8) / (result.SentSeconds * 1000000) // Convert to Mbps
	}
	if result.ReceivedSeconds > 0 {
		downloadMbps = (result.ReceivedBytes * 8) / (result.ReceivedSeconds * 1000000) // Convert to Mbps
	}

	// Calculate packet loss percentage
	var uploadLoss, downloadLoss float64
	if result.SentBytes > 0 {
		uploadLoss = ((result.SentBytes - result.ReceivedBytes) / result.SentBytes) * 100
		if uploadLoss < 0 {
			uploadLoss = 0 // No negative loss
		}
	}
	if result.ReceivedBytes > 0 {
		downloadLoss = ((result.ReceivedBytes - result.SentBytes) / result.ReceivedBytes) * 100
		if downloadLoss < 0 {
			downloadLoss = 0 // No negative loss
		}
	}

	// Calculate latency (rough estimation based on retransmits and time)
	var uploadLatency, downloadLatency float64
	if !result.UDPMode && result.SentSeconds > 0 {
		// TCP mode: estimate latency from retransmits
		uploadLatency = (result.Retransmits * 100) / result.SentSeconds       // Rough estimate in ms
		downloadLatency = (result.Retransmits * 100) / result.ReceivedSeconds // Rough estimate in ms
	} else if result.UDPMode {
		// UDP mode: use jitter as latency indicator
		uploadLatency = result.SentJitter * 1000       // Convert to ms
		downloadLatency = result.ReceivedJitter * 1000 // Convert to ms
	}

	// Emit calculated metrics
	ch <- prometheus.MustNewConstMetric(c.bandwidthUploadMbps, prometheus.GaugeValue, uploadMbps, labelValues...)
	ch <- prometheus.MustNewConstMetric(c.bandwidthDownloadMbps, prometheus.GaugeValue, downloadMbps, labelValues...)
	ch <- prometheus.MustNewConstMetric(c.packetLossUpload, prometheus.GaugeValue, uploadLoss, labelValues...)
	ch <- prometheus.MustNewConstMetric(c.packetLossDownload, prometheus.GaugeValue, downloadLoss, labelValues...)
	ch <- prometheus.MustNewConstMetric(c.latencyUpload, prometheus.GaugeValue, uploadLatency, labelValues...)
	ch <- prometheus.MustNewConstMetric(c.latencyDownload, prometheus.GaugeValue, downloadLatency, labelValues...)
}
