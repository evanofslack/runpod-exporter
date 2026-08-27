package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var AccountSSHKeys = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "runpod_account_ssh_keys",
	Help: "Number of SSH keys registered on the account.",
})
