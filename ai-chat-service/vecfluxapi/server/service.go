package server

import (
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/log"
	"ai-chat-service/vecfluxapi/llm"
	"ai-chat-service/vecfluxapi/orchestrator"
	vpb "ai-chat-service/vecfluxapi/proto"
	"context"
	"time"

	"github.com/google/uuid"
)

type VecFluxAPIServer struct {
	vpb.UnimplementedVecFluxAPIServer
	cnf           *config.Config
	log           log.ILogger
	qaStore       storage.IQAStore
	embedder      vector.Embedder
	vectorStore   vector.VectorStore
	orchestrator  *orchestrator.Orchestrator
	llm           *llm.Client
	topicAssigner TopicAssigner
}

type TopicAssigner interface {
	AssignTopic(ctx context.Context, question, qaHash string, embedding []float32) (topicID, topicName string, err error)
}

func New(
	cnf *config.Config,
	logger log.ILogger,
	qaStore storage.IQAStore,
	embedder vector.Embedder,
	vectorStore vector.VectorStore,
	orch *orchestrator.Orchestrator,
	topicAssigner TopicAssigner,
) *VecFluxAPIServer {
	return &VecFluxAPIServer{
		cnf:           cnf,
		log:           logger,
		qaStore:       qaStore,
		embedder:      embedder,
		vectorStore:   vectorStore,
		orchestrator:  orch,
		llm:           llm.NewClient(cnf),
		topicAssigner: topicAssigner,
	}
}

func (s *VecFluxAPIServer) VSet(ctx context.Context, in *vpb.VSetRequest) (*vpb.VSetResponse, error) {
	if in == nil || in.Question == "" || in.Answer == "" {
		return &vpb.VSetResponse{Ok: false}, nil
	}
	qaKey, err := s.persistQA(ctx, in.Question, in.Answer, in.Meta, 0, 0)
	if err != nil {
		s.log.Error(err)
		return nil, err
	}
	return &vpb.VSetResponse{QaKey: qaKey, Ok: true}, nil
}

func (s *VecFluxAPIServer) persistQA(ctx context.Context, question, answer string, meta *vpb.QAMeta, promptTokens, completionTokens int) (string, error) {
	normalized := storage.PrepareQueryText(question)
	qaHash := storage.QuestionHash(question)

	var emb []float32
	if s.cnf.Vector.Enabled && s.embedder != nil {
		vec, err := s.embedder.Embed(ctx, normalized)
		if err != nil {
			return "", err
		}
		emb = vec
	}

	topicID, topicName := "", ""
	if s.topicAssigner != nil && len(emb) > 0 {
		id, name, err := s.topicAssigner.AssignTopic(ctx, question, qaHash, emb)
		if err != nil {
			s.log.Error(err)
		} else {
			topicID, topicName = id, name
		}
	}

	source := storage.SourceLLM
	if meta != nil && meta.Source != "" {
		source = meta.Source
	}
	if meta != nil && meta.TopicId != "" {
		topicID = meta.TopicId
	}
	pt, ct := promptTokens, completionTokens
	if meta != nil {
		if meta.PromptTokens > 0 {
			pt = int(meta.PromptTokens)
		}
		if meta.CompletionTokens > 0 {
			ct = int(meta.CompletionTokens)
		}
	}

	record := &storage.QACacheRecord{
		Question:         question,
		Answer:           answer,
		Source:           source,
		TopicID:          topicID,
		TopicName:        topicName,
		PromptTokens:     pt,
		CompletionTokens: ct,
		TotalTokens:      pt + ct,
		CreatedAt:        time.Now().Unix(),
	}
	ttl := s.cnf.Chat.QACacheTTL
	if err := s.qaStore.SetQA(ctx, question, record, ttl); err != nil {
		return "", err
	}
	if s.cnf.Vector.Enabled && s.vectorStore != nil && len(emb) > 0 {
		if err := s.vectorStore.Build(ctx, "", normalized, emb); err != nil {
			return "", err
		}
	}
	return qaHash, nil
}

func newRequestID() string {
	return uuid.New().String()
}
