package metrics

import (
	"github.com/hashicorp/go-hclog"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics binds the registry as well as the metrics collection.
type Metrics struct {
	l      hclog.Logger
	broker string

	r *prometheus.Registry

	robotRSSI             *prometheus.GaugeVec
	robotVBat             *prometheus.GaugeVec
	robotPowerBoard       *prometheus.GaugeVec
	robotPowerPico        *prometheus.GaugeVec
	robotPowerGPIO        *prometheus.GaugeVec
	robotPowerBusA        *prometheus.GaugeVec
	robotPowerBusB        *prometheus.GaugeVec
	robotWatchdogOK       *prometheus.GaugeVec
	robotWatchdogLifetime *prometheus.GaugeVec
}

type report struct {
	VBat              int32
	WatchdogRemaining int32
	WatchdogOK        bool
	RSSI              uint8
	PwrBoard          bool
	PwrPico           bool
	PwrGPIO           bool
	PwrMainA          bool
	PwrMainB          bool
}

// Option provides a configuration framework to setup the metrics
// package.
type Option func(m *Metrics)
