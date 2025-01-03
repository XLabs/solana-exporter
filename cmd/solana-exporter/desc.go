package main

import (
	"strconv"
	"strings"

	"github.com/asymmetric-research/solana-exporter/pkg/slog"
	"github.com/prometheus/client_golang/prometheus"
)

type GaugeDesc struct {
	Desc           *prometheus.Desc
	Name           string
	Help           string
	VariableLabels []string
}

func NewGaugeDesc(name string, description string, variableLabels ...string) *GaugeDesc {
	return &GaugeDesc{
		Desc:           prometheus.NewDesc(name, description, variableLabels, nil),
		Name:           name,
		Help:           description,
		VariableLabels: variableLabels,
	}
}

func (c *GaugeDesc) MustNewConstMetric(value float64, labels ...string) prometheus.Metric {
	logger := slog.Get()
	if len(labels) != len(c.VariableLabels) {
		logger.Fatalf("Provided labels (%v) do not match %s labels (%v)", labels, c.Name, c.VariableLabels)
	}
	logger.Debugf("Emitting %v to %s(%v)", value, labels, c.Name)
	return prometheus.MustNewConstMetric(c.Desc, prometheus.GaugeValue, value, labels...)
}

func (c *GaugeDesc) NewInvalidMetric(err error) prometheus.Metric {
	return prometheus.NewInvalidMetric(c.Desc, err)
}

func parseVersionToNumber(version string) float64 {
	// Remove "v" prefix if present
	version = strings.TrimPrefix(version, "v")

	// Split version into parts
	parts := strings.Split(version, ".")

	// Convert to number
	if len(parts) >= 3 {
		major, _ := strconv.ParseFloat(parts[0], 64)
		minor, _ := strconv.ParseFloat(parts[1], 64)
		patch, _ := strconv.ParseFloat(parts[2], 64)

		return major*1e4 + minor*1e2 + patch
	}
	return 0
}

var (
	descSolanaMinRequiredVersion = prometheus.NewDesc(
		"solana_min_required_version",
		"Minimum required Solana version for foundation delegation program",
		[]string{"version", "cluster"},
		nil,
	)
)
