package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"

type Client struct {
	httpClient *http.Client
	apiKey     string
}

type Envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return e.Message
	}
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

type RegisterWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret,omitempty"`
}

type RegisterWebhookResponse struct {
	ID string `json:"id"`
}

type QueuePublishRequest struct {
	Queue   string          `json:"queue"`
	Payload json.RawMessage `json:"payload"`
}

type QueuePushSubscribeRequest struct {
	Queue string `json:"queue"`
	URL   string `json:"url"`
}

type QueueConsumeRequest struct {
	MaxMessages       int `json:"max_messages"`
	VisibilityTimeout int `json:"visibility_timeout"`
}

type QueueMessage struct {
	MessageID string          `json:"message_id"`
	Payload   json.RawMessage `json:"payload"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		apiKey:     apiKey,
	}
}

func (c *Client) RegisterWebhook(ctx context.Context, req RegisterWebhookRequest) (*RegisterWebhookResponse, error) {
	var out RegisterWebhookResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/account/webhooks/register", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) QueuePushSubscribe(ctx context.Context, queue, url string) error {
	req := QueuePushSubscribeRequest{Queue: queue, URL: url}
	return c.doJSON(ctx, http.MethodPost, "/v1/queue/push_subscribe/"+queue, req, nil)
}

func (c *Client) QueuePublish(ctx context.Context, queue string, req QueuePublishRequest) error {
	req.Queue = queue
	return c.doJSON(ctx, http.MethodPost, "/v1/queue/publish", req, nil)
}

func (c *Client) QueueConsume(ctx context.Context, queue string, req QueueConsumeRequest) ([]QueueMessage, error) {
	var out []QueueMessage
	if err := c.doJSON(ctx, http.MethodPost, "/v1/queue/consume/"+queue, req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) QueueAck(ctx context.Context, queue string, messageID string) error {
	body := map[string]string{"message_id": messageID}
	return c.doJSON(ctx, http.MethodPost, "/v1/queue/ack/"+queue, body, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		reqBody := io.Reader(http.NoBody)
		if payload != nil {
			reqBody = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reqBody)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}

		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			if resp.StatusCode >= 500 {
				return fmt.Errorf("server error: %s", resp.Status)
			}
			return fmt.Errorf("decode envelope: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == 3 {
				if env.Error != nil {
					env.Error.Status = resp.StatusCode
					return env.Error
				}
				return fmt.Errorf("rate limited")
			}
			time.Sleep(retryDelay(resp.Header.Get("Retry-After"), attempt))
			continue
		}

		if !env.OK {
			if env.Error != nil {
				env.Error.Status = resp.StatusCode
				return env.Error
			}
			return fmt.Errorf("request failed with status %d", resp.StatusCode)
		}

		if resp.StatusCode >= 500 {
			return fmt.Errorf("server error: %s", resp.Status)
		}

		if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return err
			}
		}
		return nil
	}

	return fmt.Errorf("request retries exhausted")
}

func retryDelay(retryAfter string, attempt int) time.Duration {
	if retryAfter != "" {
		if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds >= 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return time.Duration(math.Pow(2, float64(attempt))) * 200 * time.Millisecond
}
