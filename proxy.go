package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

type ValSetter interface {
	Set(float64)
}

type MetricVecProxy struct {
	mv interface{}
}

func NewMetricVecProxy(family MetricFamily, opts prometheus.Opts, labelNames []string) *MetricVecProxy {
	switch family {
	case Counter:
		return &MetricVecProxy{
			mv: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace:   opts.Namespace,
					Name:        opts.Name,
					Help:        opts.Help,
					ConstLabels: opts.ConstLabels,
				},
				labelNames,
			),
		}
	case Gauge:
		return &MetricVecProxy{
			mv: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace:   opts.Namespace,
					Name:        opts.Name,
					Help:        opts.Help,
					ConstLabels: opts.ConstLabels,
				},
				labelNames,
			),
		}
	default:
		log.Fatal("Unsupported metric family: %T", family)
	}

	return nil
}

func (p *MetricVecProxy) Collect(ch chan<- prometheus.Metric) {
	switch v := p.mv.(type) {
	case *prometheus.CounterVec:
		v.Collect(ch)
	case *prometheus.GaugeVec:
		v.Collect(ch)
	default:
		log.Fatal("Collect: Unsupported metric vector type: %T", v)
	}
}

func (p *MetricVecProxy) Describe(ch chan<- *prometheus.Desc) {
	switch v := p.mv.(type) {
	case *prometheus.CounterVec:
		v.Describe(ch)
	case *prometheus.GaugeVec:
		v.Describe(ch)
	default:
		log.Fatal("Describe: unsupported metric vector type: %T", v)
	}
}

func (p *MetricVecProxy) Reset() {
	switch v := p.mv.(type) {
	case *prometheus.CounterVec:
		v.Reset()
	case *prometheus.GaugeVec:
		v.Reset()
	default:
		log.Fatal("Reset: unsupported metric vector type: %T", v)
	}
}

func (p *MetricVecProxy) WithLabelValues(lvs ...string) ValSetter {
	switch v := p.mv.(type) {
	case *prometheus.CounterVec:
		return v.WithLabelValues(lvs...)
	case *prometheus.GaugeVec:
		return v.WithLabelValues(lvs...)
	default:
		log.Fatal("WithLabelValues: unsupported metric type: %T", v)
	}

	return nil
}
