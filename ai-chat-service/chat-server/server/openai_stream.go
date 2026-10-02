package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type chatCompletionStream interface {
	Recv() (openai.ChatCompletionStreamResponse, error)
	Close() error
}

type openAIStream struct {
	*openai.ChatCompletionStream
}

func (s *openAIStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	return s.ChatCompletionStream.Recv()
}

func (s *openAIStream) Close() error {
	return s.ChatCompletionStream.Close()
}

type sseChatStream struct {
	resp   *http.Response
	reader *bufio.Reader
}

func (a *app) createChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	client := a.getOpenaiClient()
	if !shouldDisableDeepSeekThinking(req.Model) {
		return client.CreateChatCompletion(ctx, req)
	}
	body, err := json.Marshal(deepseekChatRequest{
		ChatCompletionRequest: req,
		Thinking:              &thinkingToggle{Type: "disabled"},
	})
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	url := strings.TrimSuffix(a.openaiConf.BaseUrl, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+a.openaiConf.ApiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return openai.ChatCompletionResponse{}, fmt.Errorf("deepseek status %d: %s", resp.StatusCode, string(raw))
	}
	var out openai.ChatCompletionResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	return out, nil
}

func (a *app) createChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (chatCompletionStream, error) {
	client := a.getOpenaiClient()
	if !shouldDisableDeepSeekThinking(req.Model) {
		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			return nil, err
		}
		return &openAIStream{stream}, nil
	}
	return a.createDeepSeekChatStream(ctx, req)
}

func shouldDisableDeepSeekThinking(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(m, "deepseek-v4")
}

func (a *app) createDeepSeekChatStream(ctx context.Context, req openai.ChatCompletionRequest) (*sseChatStream, error) {
	req.Stream = true
	body, err := json.Marshal(deepseekChatRequest{
		ChatCompletionRequest: req,
		Thinking:              &thinkingToggle{Type: "disabled"},
	})
	if err != nil {
		return nil, err
	}
	url := strings.TrimSuffix(a.openaiConf.BaseUrl, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+a.openaiConf.ApiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("deepseek stream status %d: %s", resp.StatusCode, string(raw))
	}
	return &sseChatStream{resp: resp, reader: bufio.NewReader(resp.Body)}, nil
}

func (s *sseChatStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return openai.ChatCompletionStreamResponse{}, io.EOF
			}
			return openai.ChatCompletionStreamResponse{}, err
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return openai.ChatCompletionStreamResponse{}, io.EOF
		}
		var resp openai.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			return openai.ChatCompletionStreamResponse{}, err
		}
		return resp, nil
	}
}

func (s *sseChatStream) Close() error {
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}

type thinkingToggle struct {
	Type string `json:"type"`
}

type deepseekChatRequest struct {
	openai.ChatCompletionRequest
	Thinking *thinkingToggle `json:"thinking,omitempty"`
}
