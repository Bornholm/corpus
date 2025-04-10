package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	NameTotalTokens      = "total_tokens"
	NamePromptTokens     = "prompt_tokens"
	NameCompletionTokens = "completion_tokens"
	LabelModel           = "model"
	LabelType            = "type"
)

var TotalTokens = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name:      NameTotalTokens,
		Help:      "Total tokens",
		Namespace: Namespace,
	},
	[]string{LabelModel, LabelType},
)

var CompletionTokens = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name:      NameCompletionTokens,
		Help:      "Completion tokens",
		Namespace: Namespace,
	},
	[]string{LabelModel, LabelType},
)

var PromptTokens = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name:      NamePromptTokens,
		Help:      "Prompt tokens",
		Namespace: Namespace,
	},
	[]string{LabelModel, LabelType},
)
