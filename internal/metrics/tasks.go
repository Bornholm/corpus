package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	NameTotalTasks = "total_tasks"
	LabelStatus    = "status"
)

var TotalTasks = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name:      NameTotalTasks,
		Help:      "Total tasks",
		Namespace: Namespace,
	},
	[]string{LabelStatus},
)
