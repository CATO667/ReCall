package rerank

type Candidate struct {
	QAKey       string
	Question    string
	Answer      string
	TopicID     string
	VectorScore float32
}

type Result struct {
	Candidate
	FinalScore   float32
	LexicalScore float32
	TopicBoost   float32
}

type Weights struct {
	Vector  float32
	Lexical float32
	Topic   float32
}

type Config struct {
	Enabled bool
	Weights Weights
}

type Request struct {
	RawQuery        string
	NormalizedQuery string
	QueryTopicID    string
	Candidates      []Candidate
	TopK            int
}
