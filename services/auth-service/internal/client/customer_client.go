package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"voucher-flash-sale/pkg/contract"
)

type CustomerData struct {
	CustomerID string `json:"customerId"`
	Phone      string `json:"phone"`
	IsVerified bool   `json:"isVerified"`
}

type CustomerClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewCustomerClient(baseURL, apiKey string) *CustomerClient {
	return &CustomerClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *CustomerClient) GetCustomer(ctx context.Context, phone string) (*CustomerData, error) {
	reqURL := fmt.Sprintf("%s/customer/private/customers?phone=%s", c.baseURL, url.QueryEscape(phone))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("customer service unreachable: %w", err)
	}
	defer resp.Body.Close()

	var apiResp struct {
		Status       bool          `json:"status"`
		Code         string        `json:"code"`
		Message      string        `json:"message"`
		ErrorMessage *string       `json:"errorMessage"`
		Data         *CustomerData `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode customer response: %w", err)
	}

	if !apiResp.Status || apiResp.Data == nil {
		errMsg := "unknown error"
		if apiResp.ErrorMessage != nil {
			errMsg = *apiResp.ErrorMessage
		}
		return nil, fmt.Errorf("customer-service returned error: %s (code: %s)", errMsg, apiResp.Code)
	}

	return apiResp.Data, nil
}

func (c *CustomerClient) VerifyCustomer(ctx context.Context, customerID string, verifiedAt time.Time) error {
	reqURL := fmt.Sprintf("%s/customer/v1/internal/%s/verify", c.baseURL, customerID)

	bodyBytes, _ := json.Marshal(map[string]string{
		"verifiedAt": verifiedAt.Format(time.RFC3339),
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("customer service unreachable: %w", err)
	}
	defer resp.Body.Close()

	var apiResp contract.Response
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("failed to decode verify response: %w", err)
	}

	if !apiResp.Status {
		errMsg := "unknown error"
		if apiResp.ErrorMessage != nil {
			errMsg = *apiResp.ErrorMessage
		}
		return fmt.Errorf("failed to verify customer: %s", errMsg)
	}

	return nil
}
