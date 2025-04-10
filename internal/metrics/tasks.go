package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	NameTasks   = "tasks"
	LabelStatus = "status"
)

var Tasks = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name:      NameTasks,
		Help:      "Current tasks",
		Namespace: Namespace,
	},
	[]string{LabelStatus},
)
