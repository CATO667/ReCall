package server

import (
	"ai-chat-service/chat-server/rerank"
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/pkg/db/kvstore"
	"ai-chat-service/proto"
	"context"
	"fmt"
	"strings"
)

type RAGRef struct {
	Source   string
	Question string
	Answer   string
	Score    float32
	QAKey    string
}

type queryContext struct {
	raw        string
	normalized string
	embedding  []float32
}

type ragResolveResult struct {
	cacheRecord *storage.QACacheRecord
	cacheHit    bool
	ragRefs     []RAGRef
}

func prepareQueryText(q string) string {
	return storage.PrepareQueryText(q)
}

func (s *chatService) orchestrationV2Enabled() bool {
	return s.config.Vector.OrchestrationV2
}

func (s *chatService) semanticCacheThreshold() float32 {
	if s.config.Vector.CacheSimilarityThreshold > 0 {
		return s.config.Vector.CacheSimilarityThreshold
	}
	return 0.65
}

func (s *chatService) ragSimilarityThreshold() float32 {
	if s.config.Vector.SimilarityThreshold > 0 {
		return s.config.Vector.SimilarityThreshold
	}
	return 0.4
}

func (s *chatService) recallTopK() int {
	if s.config.Vector.RecallTopK > 0 {
		return s.config.Vector.RecallTopK
	}
	return 20
}

func (s *chatService) outputTopK() int {
	if s.config.Vector.TopK > 0 {
		return s.config.Vector.TopK
	}
	return 3
}

func (s *chatService) buildQueryContext(ctx context.Context, rawQuestion string) (*queryContext, error) {
	qc := &queryContext{
		raw:        rawQuestion,
		normalized: prepareQueryText(rawQuestion),
	}
	if !s.config.Vector.Enabled || s.embedder == nil {
		return qc, nil
	}
	emb, err := s.embedder.Embed(ctx, qc.normalized)
	if err != nil {
		return nil, err
	}
	qc.embedding = emb
	return qc, nil
}

func (s *chatService) resolveRAG(ctx context.Context, rawQuestion string) (*ragResolveResult, error) {
	result := &ragResolveResult{}

	if rec, err := s.tryExactQACache(ctx, rawQuestion); err != nil {
		s.log.Error(err)
	} else if rec != nil {
		s.log.InfoF("exact qa cache hit (skip embed) question=%q", rawQuestion)
		result.cacheRecord = rec
		result.cacheHit = true
		return result, nil
	}

	qc, err := s.buildQueryContext(ctx, rawQuestion)
	if err != nil {
		s.log.Error(err)
		return result, err
	}

	cacheRec, ragRefs := s.resolveSemanticQA(ctx, qc)
	if cacheRec != nil {
		result.cacheRecord = cacheRec
		result.cacheHit = true
		return result, nil
	}
	result.ragRefs = ragRefs
	return result, nil
}

func (s *chatService) tryExactQACache(ctx context.Context, rawQuestion string) (*storage.QACacheRecord, error) {
	if s.qaStore == nil {
		return nil, nil
	}
	rec, _, err := s.qaStore.GetByQuestion(ctx, rawQuestion)
	if err != nil || rec == nil || strings.TrimSpace(rec.Answer) == "" {
		return nil, err
	}
	return rec, nil
}

func (s *chatService) resolveSemanticQA(ctx context.Context, qc *queryContext) (*storage.QACacheRecord, []RAGRef) {
	if !s.config.Vector.Enabled || s.vectorStore == nil {
		return nil, nil
	}

	results := s.searchAndRerank(ctx, qc, s.recallTopK(), s.recallTopK())
	if len(results) == 0 {
		return nil, nil
	}

	hard := s.semanticCacheThreshold()
	soft := s.ragSimilarityThreshold()
	best := results[0]

	rec, err := s.qaRecordFromResult(ctx, best)
	if err != nil {
		s.log.Error(err)
		return nil, nil
	}
	if rec == nil {
		s.log.InfoF("cache kv_miss qa_key=%s rerank_score=%.4f vector_score=%.4f question=%q",
			best.QAKey, best.FinalScore, best.VectorScore, best.Question)
		if s.busMetrics != nil {
			s.busMetrics.CacheKVMissTotalCounter.Inc()
		}
	} else {
		exactHit := isExactQuestionMatch(qc.raw, best.Question)
		if best.FinalScore >= hard && s.allowSemanticCacheHit(qc.raw, best) {
			cacheKind := "semantic"
			if exactHit {
				cacheKind = "exact"
			}
			s.log.InfoF("cache hit kind=%s rerank=%.4f vector=%.4f hard=%.2f qa_key=%s",
				cacheKind, best.FinalScore, best.VectorScore, hard, best.QAKey)
			if s.busMetrics != nil {
				s.busMetrics.CacheHitTotalCounter.WithLabelValues(cacheKind).Inc()
			}
			return rec, nil
		}
		if best.FinalScore >= hard {
			s.log.InfoF("cache blocked subject mismatch rerank=%.4f vector=%.4f qa_key=%s query=%q candidate=%q",
				best.FinalScore, best.VectorScore, best.QAKey, qc.raw, best.Question)
		}
	}

	if s.orchestrationV2Enabled() && best.FinalScore >= soft {
		s.log.InfoF("semantic near-miss rerank=%.4f hard=%.2f soft=%.2f qa_key=%s",
			best.FinalScore, hard, soft, best.QAKey)
		if s.busMetrics != nil {
			s.busMetrics.CacheNearMissTotalCounter.Inc()
		}
	}
	ragRefs := ragRefsFromResults(results, soft, s.outputTopK())

	if len(ragRefs) > 0 {
		s.log.InfoF("qa rag retrieved %d refs top1_rerank=%.2f top1_vector=%.2f",
			len(ragRefs), ragRefs[0].Score, results[0].VectorScore)
	}
	s.recordRAGRefMetrics(ragRefs)
	return nil, ragRefs
}

func isExactQuestionMatch(rawQuery, candidateQuestion string) bool {
	return storage.QuestionsEquivalent(rawQuery, candidateQuestion)
}

func (s *chatService) allowSemanticCacheHit(rawQuery string, best rerank.Result) bool {
	return storage.SubjectsCompatible(rawQuery, best.Question)
}

func (s *chatService) searchAndRerank(ctx context.Context, qc *queryContext, recallTopK, rerankTopK int) []rerank.Result {
	candidates, err := s.recallQACandidates(ctx, qc, recallTopK)
	if err != nil {
		s.log.Error(err)
		return nil
	}
	if len(candidates) == 0 {
		return nil
	}

	if s.rerankService == nil || !s.rerankService.Enabled() {
		return fallbackRerank(candidates, rerankTopK)
	}

	return s.rerankService.Rerank(rerank.Request{
		RawQuery:        qc.raw,
		NormalizedQuery: qc.normalized,
		Candidates:      candidates,
		TopK:            rerankTopK,
	})
}

func (s *chatService) recallQACandidates(ctx context.Context, qc *queryContext, recallTopK int) ([]rerank.Candidate, error) {
	candidates := make([]rerank.Candidate, 0, recallTopK+1)

	exact, err := s.exactQACandidate(ctx, qc.raw)
	if err != nil {
		return nil, err
	}
	if exact != nil {
		candidates = append(candidates, *exact)
	}

	if len(qc.embedding) == 0 {
		return candidates, nil
	}
	if recallTopK <= 0 {
		recallTopK = s.recallTopK()
	}

	hits, err := s.vectorStore.SearchTopK(ctx, qc.normalized, qc.embedding, recallTopK, vector.RecallScoreFilter)
	if err != nil {
		return candidates, err
	}

	seen := make(map[string]struct{}, len(candidates)+len(hits))
	for _, c := range candidates {
		if c.QAKey != "" {
			seen[c.QAKey] = struct{}{}
		}
	}

	for _, hit := range hits {
		if hit.QAKey != "" {
			if _, ok := seen[hit.QAKey]; ok {
				continue
			}
		}
		rec, err := s.loadQAFromHit(ctx, hit)
		if err != nil {
			return nil, err
		}
		if rec == nil {
			continue
		}
		if hit.QAKey != "" {
			seen[hit.QAKey] = struct{}{}
		}
		candidates = append(candidates, rerank.Candidate{
			QAKey:       hit.QAKey,
			Question:    rec.Question,
			Answer:      rec.Answer,
			TopicID:     rec.TopicID,
			VectorScore: hit.Score,
		})
	}
	return candidates, nil
}

func (s *chatService) exactQACandidate(ctx context.Context, question string) (*rerank.Candidate, error) {
	rec, _, err := s.qaStore.GetByQuestion(ctx, question)
	if err != nil {
		return nil, err
	}
	if rec == nil || strings.TrimSpace(rec.Answer) == "" {
		return nil, nil
	}
	hash := storage.QuestionHash(question)
	return &rerank.Candidate{
		QAKey:       kvstore.QAMetaKey(hash),
		Question:    rec.Question,
		Answer:      rec.Answer,
		TopicID:     rec.TopicID,
		VectorScore: 1.0,
	}, nil
}

func fallbackRerank(candidates []rerank.Candidate, topK int) []rerank.Result {
	if len(candidates) == 0 {
		return nil
	}
	out := make([]rerank.Result, 0, len(candidates))
	for _, cand := range candidates {
		out = append(out, rerank.Result{
			Candidate:  cand,
			FinalScore: cand.VectorScore,
		})
	}
	if topK > 0 && len(out) > topK {
		out = out[:topK]
	}
	return out
}

func (s *chatService) qaRecordFromResult(ctx context.Context, result rerank.Result) (*storage.QACacheRecord, error) {
	hash := qaHashFromKey(result.QAKey)
	if hash == "" {
		return nil, nil
	}
	rec, err := s.qaStore.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	rec.SimilarityScore = result.FinalScore
	return rec, nil
}

func (s *chatService) loadQAFromHit(ctx context.Context, hit vector.RetrievalHit) (*storage.QACacheRecord, error) {
	hash := qaHashFromKey(hit.QAKey)
	if hash == "" {
		return nil, nil
	}
	return s.qaStore.GetByHash(ctx, hash)
}

func ragRefsFromResults(results []rerank.Result, minScore float32, topK int) []RAGRef {
	if len(results) == 0 {
		return nil
	}
	refs := make([]RAGRef, 0, topK)
	for _, item := range results {
		if item.FinalScore < minScore {
			continue
		}
		refs = append(refs, RAGRef{
			Source:   "qa",
			Question: item.Question,
			Answer:   item.Answer,
			Score:    item.FinalScore,
			QAKey:    item.QAKey,
		})
		if len(refs) >= topK {
			break
		}
	}
	return refs
}

func cacheTokensSaved(rec *storage.QACacheRecord) int32 {
	if rec == nil {
		return 0
	}
	if rec.TotalTokens > 0 {
		return int32(rec.TotalTokens)
	}
	return int32(rec.PromptTokens + rec.CompletionTokens)
}

func (s *chatService) recordRAGRefMetrics(refs []RAGRef) {
	if s.busMetrics == nil {
		return
	}
	for _, ref := range refs {
		source := ref.Source
		if source == "" {
			source = "qa"
		}
		s.busMetrics.RAGRefsTotalCounter.WithLabelValues(source).Inc()
	}
}

func buildRAGContextBlock(refs []RAGRef) string {
	if len(refs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n以下是可能相关的历史问答，仅供参考：\n")
	for i, ref := range refs {
		sb.WriteString(fmt.Sprintf("【参考%d】相似度：%.2f\n", i+1, ref.Score))
		sb.WriteString(fmt.Sprintf("问：%s\n", ref.Question))
		sb.WriteString(fmt.Sprintf("答：%s\n", ref.Answer))
	}
	sb.WriteString("\n请优先参考以上相似历史问答回答用户问题；若资料不足或与问题无关，可基于你的知识回答并说明。")
	return sb.String()
}

func responseSourceFromRAG(refs []RAGRef) proto.ResponseSource {
	if len(refs) > 0 {
		return proto.ResponseSource_SOURCE_RAG
	}
	return proto.ResponseSource_SOURCE_LLM
}
