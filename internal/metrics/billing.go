package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// BillingCostDollars holds the latest hourly bucket's spend, broken out by
// resource. resource is directly the BillingAmounts field names (minus the
// "Amount" suffix), snake_cased.
var BillingCostDollars = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "runpod_billing_cost_dollars",
	Help: "Latest hourly billing bucket's cost in USD, by resource.",
}, []string{"resource"})
