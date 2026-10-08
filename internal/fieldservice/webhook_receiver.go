package fieldservice

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/example/field-service-event-consumer-go/internal/infrai"
)

var ErrUnauthorized = errors.New("invalid webhook signature")

func HandleSignedEvent(r *http.Request, secret string, client *infrai.Client, queueName string) (int, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return 0, err
	}
	if err := verifySignature(body, r.Header.Get("X-Webhook-Signature"), secret); err != nil {
		return 0, err
	}
	if err := client.QueuePublish(context.Background(), queueName, infrai.QueuePublishRequest{Payload: body}); err != nil {
		return 0, err
	}
	return http.StatusAccepted, nil
}

func verifySignature(body []byte, header string, secret string) error {
	parts := strings.SplitN(header, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return ErrUnauthorized
	}
	provided, err := hex.DecodeString(parts[1])
	if err != nil {
		return ErrUnauthorized
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	if !hmac.Equal(provided, expected) {
		return ErrUnauthorized
	}
	return nil
}
