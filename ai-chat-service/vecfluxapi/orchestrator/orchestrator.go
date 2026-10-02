package orchestrator

import (
	"ai-chat-service/chat-server/rerank"
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/db/kvstore"
	"ai-chat-service/pkg/log"
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

type ResolveResult struct {
	CacheRecord *storage.QACacheRecord
	CacheHit    bool
	RAGRefs     []RAGRef
}

type Orchestrator struct {
	cnf           *config.Config
	log           log.ILogger
	qaStore       storage.IQAStore
	embedder      vector.Embedder
	vectorStore   vector.VectorStore
	rerankService *rerank.Service
}

func New(
	cnf *config.Config,
	logger log.ILogger,
	qaStore storage.IQAStore,
	embedder vector.Embedder,
	vectorStore vector.VectorStore,
	rerankService *rerank.Service,
) *Orchestrator {
	return &Orchestrator{
		cnf:           cnf,
		log:           logger,
		qaStore:       qaStore,
		embedder:      embedder,
		vectorStore:   vectorStore,
		rerankService: rerankService,
	}
}

func (o *Orchestrator) Resolve(ctx context.Context, rawQuestion string) (*ResolveResult, error) {
	result := &ResolveResult{}
	if rec, err := o.tryExactQACache(ctx, rawQuestion); err != nil {
		o.log.Error(err)
	} else if rec != nil {
		o.log.InfoF("exact qa cache hit (skip embed) question=%q", rawQuestion)
		result.CacheRecord = rec
		result.CacheHit = true
		return result, nil
	}

	qc, err := o.buildQueryContext(ctx, rawQuestion)
	if err != nil {
		o.log.Error(err)
		qc = &queryContext{
			raw:        rawQuestion,
			normalized: storage.PrepareQueryText(rawQuestion),
		}
	}

	cacheRec, ragRefs := o.resolveSemanticQA(ctx, qc)
	if cacheRec != nil {
		result.CacheRecord = cacheRec
		result.CacheHit = true
		return result, nil
	}
	result.RAGRefs = ragRefs
	return result, nil
}

func (o *Orchestrator) tryExactQACache(ctx context.Context, rawQuestion string) (*storage.QACacheRecord, error) {
	if o.qaStore == nil {
		return nil, nil
	}
	rec, _, err := o.qaStore.GetByQuestion(ctx, rawQuestion)
	if err != nil || rec == nil || strings.TrimSpace(rec.Answer) == "" {
		return nil, err
	}
	return rec, nil
}

func (o *Orchestrator) buildQueryContext(ctx context.Context, rawQuestion string) (*queryContext, error) {
	qc := &queryContext{
		raw:        rawQuestion,
		normalized: storage.PrepareQueryText(rawQuestion),
	}
	if !o.cnf.Vector.Enabled || o.embedder == nil {
		return qc, nil
	}
	emb, err := o.embedder.Embed(ctx, qc.normalized)
	if err != nil {
		return qc, err
	}
	qc.embedding = emb
	return qc, nil
}

func (o *Orchestrator) resolveSemanticQA(ctx context.Context, qc *queryContext) (*storage.QACacheRecord, []RAGRef) {
	if !o.cnf.Vector.Enabled || o.vectorStore == nil {
		return nil, nil
	}

	results := o.searchAndRerank(ctx, qc, o.recallTopK(), o.recallTopK())
	if len(results) == 0 {
		return nil, nil
	}

	hard := o.semanticCacheThreshold()
	soft := o.ragSimilarityThreshold()
	best := results[0]

	rec := o.qaRecordFromResult(ctx, best)
	if rec == nil {
		o.log.InfoF("cache kv_miss qa_key=%s rerank_score=%.4f vector_score=%.4f question=%q",
			best.QAKey, best.FinalScore, best.VectorScore, best.Question)
	} else if best.FinalScore >= hard && o.allowSemanticCacheHit(qc.raw, best) {
		o.log.InfoF("cache hit rerank=%.4f vector=%.4f hard=%.2f qa_key=%s",
			best.FinalScore, best.VectorScore, hard, best.QAKey)
		return rec, nil
	} else if rec != nil && best.FinalScore >= hard {
		o.log.InfoF("cache blocked subject mismatch rerank=%.4f vector=%.4f qa_key=%s query=%q candidate=%q",
			best.FinalScore, best.VectorScore, best.QAKey, qc.raw, best.Question)
	}

	if o.orchestrationV2Enabled() && best.FinalScore >= soft {
		o.log.InfoF("semantic near-miss rerank=%.4f hard=%.2f soft=%.2f qa_key=%s",
			best.FinalScore, hard, soft, best.QAKey)
	}

	ragRefs := ragRefsFromResults(results, soft, o.outputTopK())
	if len(ragRefs) > 0 {
		o.log.InfoF("qa rag retrieved %d refs top1_rerank=%.2f", len(ragRefs), ragRefs[0].Score)
	}
	return nil, ragRefs
}

func (o *Orchestrator) searchAndRerank(ctx context.Context, qc *queryContext, recallTopK, rerankTopK int) []rerank.Result {
	candidates, err := o.recallQACandidates(ctx, qc, recallTopK)
	if err != nil {
		o.log.Error(err)
		return nil
	}
	if len(candidates) == 0 {
		return nil
	}
	if o.rerankService == nil || !o.rerankService.Enabled() {
		return fallbackRerank(candidates, rerankTopK)
	}
	return o.rerankService.Rerank(rerank.Request{
		RawQuery:        qc.raw,
		NormalizedQuery: qc.normalized,
		Candidates:      candidates,
		TopK:            rerankTopK,
	})
}

func (o *Orchestrator) recallQACandidates(ctx context.Context, qc *queryContext, recallTopK int) ([]rerank.Candidate, error) {
	candidates := make([]rerank.Candidate, 0, recallTopK+1)
	exact, err := o.exactQACandidate(ctx, qc.raw)
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
		recallTopK = o.recallTopK()
	}
	hits, err := o.vectorStore.SearchTopK(ctx, qc.normalized, qc.embedding, recallTopK, vector.RecallScoreFilter)
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
		rec, err := o.loadQAFromHit(ctx, hit)
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

func (o *Orchestrator) exactQACandidate(ctx context.Context, question string) (*rerank.Candidate, error) {
	rec, _, err := o.qaStore.GetByQuestion(ctx, question)
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

func (o *Orchestrator) qaRecordFromResult(ctx context.Context, result rerank.Result) *storage.QACacheRecord {
	rec := qaRecordFromCandidate(result)
	if o.qaStore != nil && result.QAKey != "" {
		stored, err := o.qaStore.GetByKey(ctx, result.QAKey)
		if err != nil {
			o.log.Error(err)
		} else if stored != nil {
			rec = stored
		}
	}
	if rec == nil || strings.TrimSpace(rec.Answer) == "" {
		return nil
	}
	rec.SimilarityScore = result.FinalScore
	return rec
}

func qaRecordFromCandidate(result rerank.Result) *storage.QACacheRecord {
	if strings.TrimSpace(result.Answer) == "" {
		return nil
	}
	rec := &storage.QACacheRecord{
		Question:        result.Question,
		Answer:          result.Answer,
		TopicID:         result.TopicID,
		SimilarityScore: result.FinalScore,
	}
	return rec
}

func (o *Orchestrator) loadQAFromHit(ctx context.Context, hit vector.RetrievalHit) (*storage.QACacheRecord, error) {
	if o.qaStore == nil || hit.QAKey == "" {
		return nil, nil
	}
	hash := kvstore.ParseQAHashFromKey(hit.QAKey)
	if hash == "" {
		return nil, nil
	}
	return o.qaStore.GetByHash(ctx, hash)
}

func fallbackRerank(candidates []rerank.Candidate, topK int) []rerank.Result {
	out := make([]rerank.Result, 0, len(candidates))
	for _, cand := range candidates {
		out = append(out, rerank.Result{Candidate: cand, FinalScore: cand.VectorScore})
	}
	if topK > 0 && len(out) > topK {
		out = out[:topK]
	}
	return out
}

func ragRefsFromResults(results []rerank.Result, minScore float32, topK int) []RAGRef {
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

func BuildRAGContextBlock(refs []RAGRef) string {
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

func CacheTokensSaved(rec *storage.QACacheRecord) int32 {
	if rec == nil {
		return 0
	}
	if rec.TotalTokens > 0 {
		return int32(rec.TotalTokens)
	}
	return int32(rec.PromptTokens + rec.CompletionTokens)
}

func isExactQuestionMatch(rawQuery, candidateQuestion string) bool {
	return storage.QuestionsEquivalent(rawQuery, candidateQuestion)
}

// allowSemanticCacheHit gates semantic cache beyond rerank score.
// Subject mismatch (e.g. b+树 vs 红黑树) is blocked here; paraphrase hits rely on rerank >= hard.
func (o *Orchestrator) allowSemanticCacheHit(rawQuery string, best rerank.Result) bool {
	return storage.SubjectsCompatible(rawQuery, best.Question)
}

func (o *Orchestrator) semanticCacheThreshold() float32 {
	if o.cnf.Vector.CacheSimilarityThreshold > 0 {
		return o.cnf.Vector.CacheSimilarityThreshold
	}
	return 0.65
}

func (o *Orchestrator) ragSimilarityThreshold() float32 {
	if o.cnf.Vector.SimilarityThreshold > 0 {
		return o.cnf.Vector.SimilarityThreshold
	}
	return 0.4
}

func (o *Orchestrator) recallTopK() int {
	if o.cnf.Vector.RecallTopK > 0 {
		return o.cnf.Vector.RecallTopK
	}
	return 20
}

func (o *Orchestrator) outputTopK() int {
	if o.cnf.Vector.TopK > 0 {
		return o.cnf.Vector.TopK
	}
	return 3
}

func (o *Orchestrator) orchestrationV2Enabled() bool {
	return o.cnf.Vector.OrchestrationV2
}
