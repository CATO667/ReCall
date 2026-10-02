package kvstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type VectorHit struct {
	ID       string
	Distance float32
}

func encodeVector(vec []float32) string {
	if len(vec) == 0 {
		return ""
	}
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = strconv.FormatFloat(float64(v), 'g', -1, 32)
	}
	return strings.Join(parts, " ")
}

func parseDistance(s string) (float32, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 32)
	if err != nil {
		return 0, err
	}
	return float32(f), nil
}

func vectorWrite(ctx context.Context, args ...string) (*Reply, error) {
	pool := GetWritePool()
	client := pool.Get()
	defer pool.Put(client)
	return client.Command(ctx, args...)
}

func vectorRead(ctx context.Context, args ...string) (*Reply, error) {
	return commandReadWithFallback(ctx, args...)
}

func expectOK(reply *Reply, err error) error {
	if err != nil {
		return err
	}
	if reply.Type == '-' {
		return fmt.Errorf("%s", reply.Error)
	}
	if reply.Type == '+' && reply.Status == "OK" {
		return nil
	}
	return fmt.Errorf("unexpected reply: %+v", reply)
}

func VectorCreate(ctx context.Context, name string, dim int, metric, indexType string) error {
	if metric == "" {
		metric = "cosine"
	}
	if indexType == "" {
		indexType = "flat"
	}
	reply, err := vectorWrite(ctx, "VCREATE", name, strconv.Itoa(dim), metric, indexType)
	if err != nil {
		return err
	}
	if reply.Type == '-' {
		if strings.Contains(reply.Error, "collection exists") {
			return nil
		}
		return fmt.Errorf("%s", reply.Error)
	}
	return expectOK(reply, nil)
}

func VectorDrop(ctx context.Context, name string) error {
	reply, err := vectorWrite(ctx, "VDROP", name)
	return expectOK(reply, err)
}

func VectorInsert(ctx context.Context, name, id string, vec []float32) error {
	reply, err := vectorWrite(ctx, "VINSERT", name, id, encodeVector(vec))
	return expectOK(reply, err)
}

func VectorDelete(ctx context.Context, name, id string) error {
	reply, err := vectorWrite(ctx, "VDELETE", name, id)
	if err != nil {
		return err
	}
	if reply.Type == '-' {
		return fmt.Errorf("%s", reply.Error)
	}
	if reply.Type == ':' {
		return nil
	}
	return fmt.Errorf("unexpected VDELETE reply: %+v", reply)
}

func VectorCount(ctx context.Context, name string) (int64, error) {
	reply, err := vectorRead(ctx, "VCOUNT", name)
	if err != nil {
		return 0, err
	}
	if reply.Type == '-' {
		return 0, fmt.Errorf("%s", reply.Error)
	}
	if reply.Type == ':' {
		return reply.Integer, nil
	}
	return 0, fmt.Errorf("unexpected VCOUNT reply: %+v", reply)
}

func VectorGet(ctx context.Context, name, id string) (string, error) {
	reply, err := vectorRead(ctx, "VGET", name, id)
	if err != nil {
		return "", err
	}
	if reply.Type == '-' {
		return "", fmt.Errorf("%s", reply.Error)
	}
	if reply.Type != '*' || len(reply.Array) < 1 {
		return "", fmt.Errorf("unexpected VGET reply: %+v", reply)
	}
	vecReply := reply.Array[0]
	if vecReply.Type == '$' && vecReply.IsNil {
		return "", fmt.Errorf("vector not found: %s", id)
	}
	if vecReply.Type != '$' {
		return "", fmt.Errorf("unexpected VGET vector reply: %+v", vecReply)
	}
	return vecReply.BulkString(), nil
}

func VectorSearch(ctx context.Context, name string, topK int, query []float32) ([]VectorHit, error) {
	reply, err := vectorRead(ctx, "VSEARCH", name, strconv.Itoa(topK), encodeVector(query))
	if err != nil {
		return nil, err
	}
	if reply.Type == '-' {
		return nil, fmt.Errorf("%s", reply.Error)
	}
	if reply.Type != '*' {
		return nil, fmt.Errorf("unexpected VSEARCH reply: %+v", reply)
	}
	if len(reply.Array)%2 != 0 {
		return nil, fmt.Errorf("invalid VSEARCH result length: %d", len(reply.Array))
	}
	hits := make([]VectorHit, 0, len(reply.Array)/2)
	for i := 0; i < len(reply.Array); i += 2 {
		idReply := reply.Array[i]
		scReply := reply.Array[i+1]
		if idReply.Type != '$' || scReply.Type != '$' {
			return nil, fmt.Errorf("unexpected VSEARCH element types")
		}
		dist, err := parseDistance(scReply.BulkString())
		if err != nil {
			return nil, fmt.Errorf("parse distance: %w", err)
		}
		hits = append(hits, VectorHit{
			ID:       idReply.BulkString(),
			Distance: dist,
		})
	}
	return hits, nil
}

func CosineSimilarity(distance float32) float32 {
	return 1 - distance
}
