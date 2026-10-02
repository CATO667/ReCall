package metrics_bus

import "github.com/prometheus/client_golang/prometheus"

type BusMetrics struct {
	QuestionsTotalCounter          prometheus.Counter
	KeywordsQuestionsTotalCounter  prometheus.Counter
	SensitiveQuestionsTotalCounter prometheus.Counter
	ErrQuestionsTotalCounter       prometheus.Counter
	CacheHitTotalCounter           *prometheus.CounterVec
	CacheNearMissTotalCounter      prometheus.Counter
	CacheKVMissTotalCounter        prometheus.Counter
	RAGRefsTotalCounter            *prometheus.CounterVec
}

const (
	NAMESPACE = "ai_chat"
	SUBSYSTEM = "chat_service"
)

func NewBusMetrics(registry *prometheus.Registry) *BusMetrics {
	questionsTotalCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   NAMESPACE,
		Subsystem:   SUBSYSTEM,
		Name:        "questions_total",
		ConstLabels: map[string]string{"app": "ai_chat"},
		Help:        "记录用户提交问题的总数，仅包含记录到DB的问题数量",
	})
	keywordsQuestionsTotalCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   NAMESPACE,
		Subsystem:   SUBSYSTEM,
		Name:        "keywords_questions_total",
		ConstLabels: map[string]string{"app": "ai_chat"},
		Help:        "记录用户提交的包含关键词的问题总数",
	})
	sensitiveQuestionsTotalCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   NAMESPACE,
		Subsystem:   SUBSYSTEM,
		Name:        "sensitive_questions_total",
		ConstLabels: map[string]string{"app": "ai_chat"},
		Help:        "记录用户提交的触发敏感词的问题总数",
	})
	errQuestionsTotalCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   NAMESPACE,
		Subsystem:   SUBSYSTEM,
		Name:        "err_questions_total",
		ConstLabels: map[string]string{"app": "ai_chat"},
		Help:        "记录用户提交问题时报错的总数",
	})
	cacheHitTotalCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   NAMESPACE,
		Subsystem:   SUBSYSTEM,
		Name:        "cache_hit_total",
		ConstLabels: map[string]string{"app": "ai_chat"},
		Help:        "QA 缓存命中次数，kind=exact|semantic",
	}, []string{"kind"})
	cacheNearMissTotalCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   NAMESPACE,
		Subsystem:   SUBSYSTEM,
		Name:        "cache_near_miss_total",
		ConstLabels: map[string]string{"app": "ai_chat"},
		Help:        "语义向量命中 soft 区间但未直返硬缓存",
	})
	cacheKVMissTotalCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   NAMESPACE,
		Subsystem:   SUBSYSTEM,
		Name:        "cache_kv_miss_total",
		ConstLabels: map[string]string{"app": "ai_chat"},
		Help:        "向量命中但 KV 无 QA 记录",
	})
	ragRefsTotalCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   NAMESPACE,
		Subsystem:   SUBSYSTEM,
		Name:        "rag_refs_total",
		ConstLabels: map[string]string{"app": "ai_chat"},
		Help:        "RAG 检索引用来源计数",
	}, []string{"source"})
	registry.MustRegister(
		questionsTotalCounter,
		keywordsQuestionsTotalCounter,
		sensitiveQuestionsTotalCounter,
		errQuestionsTotalCounter,
		cacheHitTotalCounter,
		cacheNearMissTotalCounter,
		cacheKVMissTotalCounter,
		ragRefsTotalCounter,
	)
	return &BusMetrics{
		QuestionsTotalCounter:          questionsTotalCounter,
		KeywordsQuestionsTotalCounter:  keywordsQuestionsTotalCounter,
		SensitiveQuestionsTotalCounter: sensitiveQuestionsTotalCounter,
		ErrQuestionsTotalCounter:       errQuestionsTotalCounter,
		CacheHitTotalCounter:           cacheHitTotalCounter,
		CacheNearMissTotalCounter:      cacheNearMissTotalCounter,
		CacheKVMissTotalCounter:        cacheKVMissTotalCounter,
		RAGRefsTotalCounter:            ragRefsTotalCounter,
	}
}
