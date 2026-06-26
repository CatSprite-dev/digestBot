package digest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/CatSprite-dev/digestBot/internal/model"
)

type Digest struct {
	baseURL string
	apiKey  string
	model   string
	logger  *slog.Logger
}

type request struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type response struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

func NewDigest(baseURL, apiKey, model string, logger *slog.Logger) *Digest {
	return &Digest{baseURL: baseURL, apiKey: apiKey, model: model, logger: logger}
}

func (d *Digest) Generate(ctx context.Context, messages []model.Message) (string, error) {
	prompt := d.buildPrompt(messages)
	url := d.baseURL + "/chat/completions"

	payload := request{
		Model: d.model,
		Messages: []message{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+d.apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var resp response
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	d.logger.Debug("digest generated", "model", d.model, "messages", len(messages))
	return resp.Choices[0].Message.Content, nil
}

func (d *Digest) buildPrompt(messages []model.Message) string {
	var sb strings.Builder
	sb.WriteString("Summarize the following chat messages in Russian. Focus on key topics and decisions.\n\n")
	for _, msg := range messages {
		sb.WriteString(msg.Sender + ": " + msg.Text + "\n")
	}
	return sb.String()
}
