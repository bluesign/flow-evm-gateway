package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type Collector interface {
	ApiErrorOccurred()
	TraceDownloadFailed()
	ServerPanicked(reason string)
	EVMHeightIndexed(height uint64)
	EVMAccountInteraction(address string)
	MeasureRequestDuration(start time.Time, method string)
}

var _ Collector = &DefaultCollector{}

type DefaultCollector struct {
	// TODO: for now we cannot differentiate which api request failed number of times
	apiErrorsCounter          prometheus.Counter
	traceDownloadErrorCounter prometheus.Counter
	serverPanicsCounters      *prometheus.CounterVec
	evmBlockHeight            prometheus.Gauge
	evmAccountCallCounters    *prometheus.CounterVec
	requestDurations          *prometheus.HistogramVec
}

func NewCollector(logger zerolog.Logger) Collector {
	apiErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: prefixedName("api_errors_total"),
		Help: "Total number of API errors",
	})

	traceDownloadErrorCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: prefixedName("trace_download_errors_total"),
		Help: "Total number of trace download errors",
	})

	serverPanicsCounters := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prefixedName("api_server_panics_total"),
		Help: "Total number of panics in the API server",
	}, []string{"reason"})

	evmBlockHeight := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: prefixedName("evm_block_height"),
		Help: "Current EVM block height",
	})

	evmAccountCallCounters := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prefixedName("evm_account_interactions_total"),
		Help: "Total number of account interactions",
	}, []string{"address"})

	// TODO: Think of adding 'status_code'
	requestDurations := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    prefixedName("api_request_duration_seconds"),
		Help:    "Duration of the request made a specific API endpoint",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})

	metrics := []prometheus.Collector{
		apiErrors,
		traceDownloadErrorCounter,
		serverPanicsCounters,
		evmBlockHeight,
		evmAccountCallCounters,
		requestDurations,
	}
	if err := registerMetrics(logger, metrics...); err != nil {
		logger.Info().Msg("using noop collector as metric register failed")
		return NopCollector
	}

	return &DefaultCollector{
		apiErrorsCounter:          apiErrors,
		traceDownloadErrorCounter: traceDownloadErrorCounter,
		serverPanicsCounters:      serverPanicsCounters,
		evmBlockHeight:            evmBlockHeight,
		evmAccountCallCounters:    evmAccountCallCounters,
		requestDurations:          requestDurations,
	}
}

func registerMetrics(logger zerolog.Logger, metrics ...prometheus.Collector) error {
	for _, m := range metrics {
		if err := prometheus.Register(m); err != nil {
			logger.Err(err).Msg("failed to register metric")
			return err
		}
	}

	return nil
}

func (c *DefaultCollector) ApiErrorOccurred() {
	c.apiErrorsCounter.Inc()
}

func (c *DefaultCollector) TraceDownloadFailed() {
	c.traceDownloadErrorCounter.Inc()
}

func (c *DefaultCollector) ServerPanicked(reason string) {
	c.serverPanicsCounters.With(prometheus.Labels{"reason": reason}).Inc()
}

func (c *DefaultCollector) EVMHeightIndexed(height uint64) {
	c.evmBlockHeight.Set(float64(height))
}

func (c *DefaultCollector) EVMAccountInteraction(address string) {
	c.evmAccountCallCounters.With(prometheus.Labels{"address": address}).Inc()

}

func (c *DefaultCollector) MeasureRequestDuration(start time.Time, method string) {
	c.requestDurations.
		With(prometheus.Labels{"method": method}).
		Observe(time.Since(start).Seconds())
}

func prefixedName(name string) string {
	return fmt.Sprintf("evm_gateway_%s", name)
}
