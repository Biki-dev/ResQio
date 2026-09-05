package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Candidate struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Category string `json:"category,omitempty"`
}

type MatchResult struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	Category string  `json:"category"`
	Score    float64 `json:"score"`
}

type matchResponse struct {
	Query       string        `json:"query"`
	Matches     []MatchResult `json:"matches"`
	QueryVector []float32     `json:"query_vector"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Dimension  int         `json:"dimension"`
}

type upsertItem struct {
	ID       string                 `json:"id"`
	Text     string                 `json:"text"`
	Category string                 `json:"category,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:8085"
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 4 * time.Second,
		},
	}
}

func (c *Client) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	payload, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding service returned status %d", resp.StatusCode)
	}

	var res embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	if len(res.Embeddings) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}
	return res.Embeddings[0], nil
}

func (c *Client) UpsertResource(ctx context.Context, id, text, category string, metadata map[string]interface{}) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"items": []upsertItem{
			{
				ID:       id,
				Text:     text,
				Category: category,
				Metadata: metadata,
			},
		},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/upsert", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) Match(ctx context.Context, query string, topK int, candidates []Candidate) ([]MatchResult, []float32, error) {
	body := map[string]interface{}{
		"query":      query,
		"top_k":      topK,
		"candidates": candidates,
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/match", bytes.NewReader(payload))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("matching service returned status %d", resp.StatusCode)
	}

	var res matchResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, nil, err
	}
	return res.Matches, res.QueryVector, nil
}
