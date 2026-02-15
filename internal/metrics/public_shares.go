package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	NamePublicShareTotalQuestions      = "public_share_total_questions"
	NamePublicShareSucceededQuestions  = "public_share_succeeded_questions"
	NamePublicShareFailedQuestions     = "public_share_failed_questions"
	NamePublicShareUnansweredQuestions = "public_share_unanswered_questions"
	LabelPublicShareID                 = "public_share_id"
)

var PublicShareTotalQuestions = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name:      NamePublicShareTotalQuestions,
		Help:      "Public share total questions",
		Namespace: Namespace,
	},
	[]string{LabelPublicShareID},
)

var PublicShareSucceededQuestions = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name:      NamePublicShareSucceededQuestions,
		Help:      "Public share succeeded questions",
		Namespace: Namespace,
	},
	[]string{LabelPublicShareID},
)

var PublicShareFailedQuestions = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name:      NamePublicShareFailedQuestions,
		Help:      "Public share failed questions",
		Namespace: Namespace,
	},
	[]string{LabelPublicShareID},
)

var PublicShareUnansweredQuestions = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name:      NamePublicShareUnansweredQuestions,
		Help:      "Public share unanswered questions",
		Namespace: Namespace,
	},
	[]string{LabelPublicShareID},
)
