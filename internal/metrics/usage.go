package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	NameTotalAskRequests    = "total_ask_requests"
	NameTotalIndexRequests  = "total_index_requests"
	NameTotalSearchRequests = "total_search_requests"
)

var TotalAskRequests = promauto.NewCounter(
	prometheus.CounterOpts{
		Name:      NameTotalAskRequests,
		Help:      "Total ask requests",
		Namespace: Namespace,
	},
)

var TotalIndexRequests = promauto.NewCounter(
	prometheus.CounterOpts{
		Name:      NameTotalIndexRequests,
		Help:      "Total index requests",
		Namespace: Namespace,
	},
)

var TotalSearchRequests = promauto.NewCounter(
	prometheus.CounterOpts{
		Name:      NameTotalSearchRequests,
		Help:      "Total search requests",
		Namespace: Namespace,
	},
)
