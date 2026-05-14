package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type UserIntegration struct {
	baseURL string
	client  *http.Client
}

func NewUserIntegration(baseURL string) *UserIntegration {
	return &UserIntegration{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (u *UserIntegration) GetUserEmail(ctx context.Context, userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", fmt.Errorf("user id is required")
	}
	if u.baseURL == "" {
		return "", fmt.Errorf("user service url is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.baseURL+"/users/"+userID, nil)
	if err != nil {
		return "", err
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("user-service returned status: %d", resp.StatusCode)
	}

	var out userResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}

	if strings.TrimSpace(out.Email) == "" {
		return "", fmt.Errorf("user email is empty")
	}

	return out.Email, nil
}
