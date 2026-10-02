package llm

import (
	"ai-chat-service/pkg/config"
	"ai-chat-service/services/tokenizer"
	"ai-chat-service/vecfluxapi/orchestrator"
	vpb "ai-chat-service/vecfluxapi/proto"
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

const chatPrimedTokens = 2

type Options struct {
	Model             string
	MaxTokens         int
	MinResponseTokens int
	Temperature       float32
	TopP              float32
	PresencePenalty   float32
	FrequencyPenalty  float32
	BotDesc           string
}

type Stream interface {
	Recv() (openai.ChatCompletionStreamResponse, error)
	Close() error
}

type Client struct {
	apiKey  string
	baseURL string
	model   string
}

func NewClient(cnf *config.Config) *Client {
	return &Client{
		apiKey:  cnf.Chat.ApiKey,
		baseURL: cnf.Chat.BaseUrl,
		model:   cnf.Chat.Model,
	}
}

func OptionsFromConfig(cnf *config.Config, in *vpb.LLMOptions) Options {
	opt := Options{
		Model:             cnf.Chat.Model,
		MaxTokens:         cnf.Chat.MaxTokens,
		MinResponseTokens: cnf.Chat.MinResponseTokens,
		Temperature:       cnf.Chat.Temperature,
		TopP:              cnf.Chat.TopP,
		PresencePenalty:   cnf.Chat.PresencePenalty,
		FrequencyPenalty:  cnf.Chat.FrequencyPenalty,
		BotDesc:           cnf.Chat.BotDesc,
	}
	if in == nil {
		return opt
	}
	if in.Model != "" {
		opt.Model = in.Model
	}
	if in.MaxTokens > 0 {
		opt.MaxTokens = int(in.MaxTokens)
	}
	if in.MinResponseTokens > 0 {
		opt.MinResponseTokens = int(in.MinResponseTokens)
	}
	if in.Temperature != 0 {
		opt.Temperature = in.Temperature
	}
	if in.TopP != 0 {
		opt.TopP = in.TopP
	}
	if in.PresencePenalty != 0 {
		opt.PresencePenalty = in.PresencePenalty
	}
	if in.FrequencyPenalty != 0 {
		opt.FrequencyPenalty = in.FrequencyPenalty
	}
	if in.BotDesc != "" {
		opt.BotDesc = in.BotDesc
	}
	return opt
}

func (c *Client) BuildRequest(question string, ctxMsgs []*vpb.ChatMessage, ragRefs []orchestrator.RAGRef, opt Options, stream bool) (openai.ChatCompletionRequest, int, error) {
	curr := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: question}
	req := openai.ChatCompletionRequest{
		Model:            opt.Model,
		Messages:         []openai.ChatCompletionMessage{curr},
		MaxTokens:        opt.MinResponseTokens,
		Temperature:      opt.Temperature,
		TopP:             opt.TopP,
		PresencePenalty:  opt.PresencePenalty,
		FrequencyPenalty: opt.FrequencyPenalty,
		Stream:           stream,
	}

	ragBlock := orchestrator.BuildRAGContextBlock(ragRefs)
	var sys openai.ChatCompletionMessage
	botTokens := 0
	if opt.BotDesc != "" || ragBlock != "" {
		sys = openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: opt.BotDesc + ragBlock,
		}
		var err error
		botTokens, err = tokenizer.GetTokens(&sys, opt.Model)
		if err != nil {
			return req, 0, err
		}
	}

	messages := []openai.ChatCompletionMessage{curr}
	currTokens, err := tokenizer.GetTokens(&curr, opt.Model)
	if err != nil {
		return req, 0, err
	}
	if currTokens > opt.MaxTokens-opt.MinResponseTokens-botTokens-chatPrimedTokens {
		return req, 0, fmt.Errorf("请求消息超限")
	}

	tokens := currTokens + botTokens + chatPrimedTokens
	for _, m := range ctxMsgs {
		if m == nil || strings.TrimSpace(m.Content) == "" {
			continue
		}
		role := m.Role
		if role == "" {
			role = openai.ChatMessageRoleUser
		}
		item := openai.ChatCompletionMessage{Role: role, Content: m.Content}
		itemTokens, err := tokenizer.GetTokens(&item, opt.Model)
		if err != nil {
			return req, 0, err
		}
		if tokens+itemTokens+chatPrimedTokens > opt.MaxTokens-opt.MinResponseTokens {
			break
		}
		messages = append(messages, item)
		tokens += itemTokens + chatPrimedTokens
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	if botTokens > 0 {
		messages = append([]openai.ChatCompletionMessage{sys}, messages...)
		tokens = botTokens + chatPrimedTokens
		for _, item := range messages[1:] {
			itemTokens, err := tokenizer.GetTokens(&item, opt.Model)
			if err != nil {
				return req, 0, err
			}
			tokens += itemTokens + chatPrimedTokens
		}
	}
	req.Messages = messages
	req.MaxTokens = effectiveMaxTokens(opt, tokens)
	return req, tokens, nil
}

func effectiveMaxTokens(opt Options, promptTokens int) int {
	remaining := opt.MaxTokens - promptTokens
	if remaining <= 0 {
		return 1
	}
	return remaining
}

func (c *Client) CreateStream(ctx context.Context, req openai.ChatCompletionRequest) (Stream, error) {
	if !shouldDisableDeepSeekThinking(req.Model) {
		cfg := openai.DefaultConfig(c.apiKey)
		cfg.BaseURL = c.baseURL
		client := openai.NewClientWithConfig(cfg)
		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			return nil, err
		}
		return &openAIStream{stream}, nil
	}
	return c.createDeepSeekStream(ctx, req)
}

type openAIStream struct{ *openai.ChatCompletionStream }

func (s *openAIStream) Close() error { return s.ChatCompletionStream.Close() }

type sseChatStream struct {
	resp   *http.Response
	reader *bufio.Reader
}

func (c *Client) createDeepSeekStream(ctx context.Context, req openai.ChatCompletionRequest) (*sseChatStream, error) {
	req.Stream = true
	body, err := json.Marshal(deepseekChatRequest{
		ChatCompletionRequest: req,
		Thinking:              &thinkingToggle{Type: "disabled"},
	})
	if err != nil {
		return nil, err
	}
	url := strings.TrimSuffix(c.baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
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

func shouldDisableDeepSeekThinking(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(m, "deepseek-v4")
}

type thinkingToggle struct {
	Type string `json:"type"`
}

type deepseekChatRequest struct {
	openai.ChatCompletionRequest
	Thinking *thinkingToggle `json:"thinking,omitempty"`
}

func DeltaText(delta openai.ChatCompletionStreamChoiceDelta) string {
	return delta.Content
}

// CompletionReadyForCache reports whether a streamed LLM response finished normally enough to cache.
func CompletionReadyForCache(ctx context.Context, finishReason, completion string) bool {
	if ctx.Err() != nil {
		return false
	}
	if strings.TrimSpace(completion) == "" {
		return false
	}
	switch finishReason {
	case "stop", "length":
		return true
	default:
		return false
	}
}
