package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var TemplateInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "runpod_template_info",
	Help: "Always 1. Template inventory/info labels.",
}, []string{"template_id", "name", "serverless", "public", "category"})
