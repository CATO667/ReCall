package vector

import (
	"ai-chat-service/pkg/config"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type openAIEmbedder struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewSiliconFlowEmbedder(cnf *config.Config) Embedder {
	baseURL := cnf.Embedder.BaseUrl
	if baseURL == "" {
		baseURL = "https://api.siliconflow.cn/v1"
	}
	model := cnf.Embedder.Model
	if model == "" {
		model = "BAAI/bge-m3"
	}
	return &openAIEmbedder{
		apiKey:  cnf.Embedder.ApiKey,
		baseURL: trimSuffixSlash(baseURL),
		model:   model,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func trimSuffixSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

type embeddingRequest struct {
	Model          string   `json:"model"`
	Input          any      `json:"input"`
	EncodingFormat string   `json:"encoding_format"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (e *openAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("embedder api_key is empty")
	}
	body, err := json.Marshal(embeddingRequest{
		Model:          e.model,
		Input:          text,
		EncodingFormat: "float",
	})
	if err != nil {
		return nil, err
	}
	url := e.baseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding api status %d: %s", resp.StatusCode, string(raw))
	}
	var out embeddingResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Error != nil && out.Error.Message != "" {
		return nil, fmt.Errorf("embedding api error: %s", out.Error.Message)
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedding api returned empty vector")
	}
	vec := make([]float32, len(out.Data[0].Embedding))
	for i, v := range out.Data[0].Embedding {
		vec[i] = float32(v)
	}
	return vec, nil
}

// EmbedBatch 一次请求嵌入多条文本，降低 RPM；遇 429 自动退避重试。
func (e *openAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if e.apiKey == "" {
		return nil, fmt.Errorf("embedder api_key is empty")
	}
	body, err := json.Marshal(embeddingRequest{
		Model:          e.model,
		Input:          texts,
		EncodingFormat: "float",
	})
	if err != nil {
		return nil, err
	}
	url := e.baseURL + "/embeddings"
	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		if attempt > 0 {
			wait := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := e.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("embedding api status 429: %s", string(raw))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("embedding api status %d: %s", resp.StatusCode, string(raw))
		}
		var out embeddingResponse
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		if out.Error != nil && out.Error.Message != "" {
			return nil, fmt.Errorf("embedding api error: %s", out.Error.Message)
		}
		if len(out.Data) != len(texts) {
			return nil, fmt.Errorf("embedding api returned %d vectors, want %d", len(out.Data), len(texts))
		}
		vectors := make([][]float32, len(out.Data))
		for i, item := range out.Data {
			if len(item.Embedding) == 0 {
				return nil, fmt.Errorf("embedding api returned empty vector at %d", i)
			}
			vec := make([]float32, len(item.Embedding))
			for j, v := range item.Embedding {
				vec[j] = float32(v)
			}
			vectors[i] = vec
		}
		return vectors, nil
	}
	return nil, lastErr
}
