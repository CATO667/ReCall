package vector

import "context"

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type RetrievalHit struct {
	QAKey    string
	Score    float32
	Question string
}

// ScoreFilter 控制向量检索时的相似度下限。
// UseDefaultScoreFilter 使用配置中的 similarity_threshold；
// RecallScoreFilter 不在向量库侧过滤，由编排层按场景再判阈值。
type ScoreFilter float32

const (
	UseDefaultScoreFilter ScoreFilter = 0
	RecallScoreFilter     ScoreFilter = -1
)

func (f ScoreFilter) Resolve(defaultThreshold float32) (threshold float32, apply bool) {
	if f == RecallScoreFilter {
		return 0, false
	}
	if f > 0 {
		return float32(f), true
	}
	return defaultThreshold, true
}

type VectorStore interface {
	Build(ctx context.Context, key string, question string, vector []float32) error
	SearchTopK(ctx context.Context, question string, vector []float32, topK int, scoreFilter ScoreFilter) ([]RetrievalHit, error)
}
