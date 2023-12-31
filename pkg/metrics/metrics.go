package metrics

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/hashicorp/go-hclog"
	"github.com/prometheus/client_golang/prometheus"
)

// New returns an initialized instance of the metrics system.
func New(opts ...Option) *Metrics {
	x := &Metrics{
		l:      hclog.NewNullLogger(),
		r:      prometheus.NewRegistry(),
		broker: "mqtt://127.0.0.1:1883",

		robotRSSI: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "best",
			Subsystem: "robot",
			Name:      "rssi",
			Help:      "WiFi signal strength as measured by the system processor.",
		}, []string{"team"}),

		robotVBat: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "best",
			Subsystem: "robot",
			Name:      "battery_voltage",
			Help:      "Robot Battery volage.",
		}, []string{"team"}),

		robotPowerBoard: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "best",
			Subsystem: "robot",
			Name:      "power_board",
			Help:      "General logic power available.",
		}, []string{"team"}),

		robotPowerPico: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "best",
			Subsystem: "robot",
			Name:      "power_pico",
			Help:      "Pico power supply available.",
		}, []string{"team"}),

		robotPowerGPIO: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "best",
			Subsystem: "robot",
			Name:      "power_gpio",
			Help:      "GPIO power supply available.",
		}, []string{"team"}),

		robotPowerBusA: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "best",
			Subsystem: "robot",
			Name:      "power_bus_a",
			Help:      "Motor Bus A power available.",
		}, []string{"team"}),

		robotPowerBusB: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "best",
			Subsystem: "robot",
			Name:      "power_bus_b",
			Help:      "Motor Bus B power available.",
		}, []string{"team"}),

		robotWatchdogOK: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "best",
			Subsystem: "robot",
			Name:      "watchdog_ok",
			Help:      "Watchdog has been fed and is alive.",
		}, []string{"team"}),

		robotWatchdogLifetime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "best",
			Subsystem: "robot",
			Name:      "watchdog_remaining_milliseconds",
			Help:      "Watchdog lifetime remaining since last feed.",
		}, []string{"team"}),
	}

	x.r.MustRegister(x.robotRSSI)
	x.r.MustRegister(x.robotVBat)
	x.r.MustRegister(x.robotPowerBoard)
	x.r.MustRegister(x.robotPowerPico)
	x.r.MustRegister(x.robotPowerGPIO)
	x.r.MustRegister(x.robotPowerBusA)
	x.r.MustRegister(x.robotPowerBusB)
	x.r.MustRegister(x.robotWatchdogOK)
	x.r.MustRegister(x.robotWatchdogLifetime)

	for _, o := range opts {
		o(x)
	}

	return x
}

// Registry provides access to the registry that this instance
// manages.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.r
}

// ResetRobotMetrics clears all metrics associated with robots and
// resets the built-in exporter to a clean state.
func (m *Metrics) ResetRobotMetrics() {
	m.robotRSSI.Reset()
	m.robotVBat.Reset()
	m.robotPowerBoard.Reset()
	m.robotPowerPico.Reset()
	m.robotPowerGPIO.Reset()
	m.robotPowerBusA.Reset()
	m.robotPowerBusB.Reset()
	m.robotWatchdogOK.Reset()
	m.robotWatchdogLifetime.Reset()
}

func (m *Metrics) mqttCallback(c mqtt.Client, msg mqtt.Message) {
	teamNum := strings.Split(msg.Topic(), "/")[1]
	m.l.Trace("Called back", "team", teamNum)
	var stats report
	if err := json.Unmarshal(msg.Payload(), &stats); err != nil {
		m.l.Warn("Bad stats report", "team", teamNum, "error", err)
	}

	m.robotRSSI.With(prometheus.Labels{"team": teamNum}).Set(float64(stats.RSSI))
	m.robotVBat.With(prometheus.Labels{"team": teamNum}).Set(float64(stats.VBat))
	m.robotWatchdogLifetime.With(prometheus.Labels{"team": teamNum}).Set(float64(stats.WatchdogRemaining))

	m.robotPowerBoard.With(prometheus.Labels{"team": teamNum}).Set(fCast(stats.PwrBoard))
	m.robotPowerPico.With(prometheus.Labels{"team": teamNum}).Set(fCast(stats.PwrPico))
	m.robotPowerGPIO.With(prometheus.Labels{"team": teamNum}).Set(fCast(stats.PwrGPIO))
	m.robotPowerBusA.With(prometheus.Labels{"team": teamNum}).Set(fCast(stats.PwrMainA))
	m.robotPowerBusB.With(prometheus.Labels{"team": teamNum}).Set(fCast(stats.PwrMainB))
	m.robotWatchdogOK.With(prometheus.Labels{"team": teamNum}).Set(fCast(stats.WatchdogOK))
}

// MQTTInit connects to the mqtt server and listens for metrics.
func (m *Metrics) MQTTInit(wg *sync.WaitGroup) error {
	wg.Add(1)
	opts := mqtt.NewClientOptions().
		AddBroker(m.broker).
		SetAutoReconnect(true).
		SetClientID("self-metrics").
		SetConnectRetry(true).
		SetConnectTimeout(time.Second).
		SetConnectRetryInterval(time.Second)
	client := mqtt.NewClient(opts)
	if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
		m.l.Error("Error connecting to broker", "error", tok.Error())
		return tok.Error()
	}
	m.l.Info("Connected to broker")

	subFunc := func() error {
		if tok := client.Subscribe("robot/+/stats", 1, m.mqttCallback); tok.Wait() && tok.Error() != nil {
			m.l.Warn("Error subscribing to topic", "error", tok.Error())
			return tok.Error()
		}
		return nil
	}
	if err := backoff.Retry(subFunc, backoff.NewExponentialBackOff()); err != nil {
		m.l.Error("Permanent error encountered while subscribing", "error", err)
		return err
	}
	m.l.Info("Subscribed to topics")
	wg.Done()
	return nil

}

func fCast(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
