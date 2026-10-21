package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type CreateCampaignResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ID         string `json:"id"`
		TotalStock int    `json:"total_stock"`
	} `json:"data"`
	Error string `json:"error"`
}

type ClaimResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

func main() {
	campaignHost := flag.String("campaign-host", "http://localhost:8081", "Campaign service host")
	claimHost := flag.String("claim-host", "http://localhost:8082", "Claim service host")
	voucherHost := flag.String("voucher-host", "http://localhost:8083", "Voucher service host")
	totalStock := flag.Int("stock", 50, "Initial stock for the test campaign")
	totalUsers := flag.Int("users", 300, "Number of distinct simulated users")
	duplicateAttempts := flag.Int("dup-factor", 2, "Duplicate claim attempts per user")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Println("🚀 VOUCHER FLASH SALE CONCURRENCY STRESS TEST")
	fmt.Println("==================================================")
	fmt.Printf("Initial Stock:      %d\n", *totalStock)
	fmt.Printf("Simulated Users:    %d\n", *totalUsers)
	fmt.Printf("Total Requests:     %d\n", (*totalUsers)*(*duplicateAttempts))
	fmt.Println("--------------------------------------------------")

	// 1. Create Campaign
	fmt.Printf("Creating test flash sale campaign with %d stock...\n", *totalStock)
	campaignID, err := createTestCampaign(*campaignHost, *totalStock)
	if err != nil {
		fmt.Printf("❌ Failed to create campaign: %v\n", err)
		return
	}
	fmt.Printf("✅ Campaign created with ID: %s\n\n", campaignID)

	// 2. High-concurrency claim barrage
	totalReqs := (*totalUsers) * (*duplicateAttempts)
	var (
		status200 int64
		status409 int64 // Duplicate
		status410 int64 // Sold out
		otherErrs int64
	)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        500,
			MaxIdleConnsPerHost: 500,
		},
	}

	start := time.Now()
	var wg sync.WaitGroup

	// Channel of user IDs
	userChan := make(chan string, totalReqs)
	for i := 1; i <= *totalUsers; i++ {
		userID := fmt.Sprintf("user-%04d", i)
		for j := 0; j < *duplicateAttempts; j++ {
			userChan <- userID
		}
	}
	close(userChan)

	concurrencyWorkers := 50
	for w := 0; w < concurrencyWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for uID := range userChan {
				claimURL := fmt.Sprintf("%s/api/v1/campaigns/%s/claim", *claimHost, campaignID)
				body, _ := json.Marshal(map[string]string{"user_id": uID})

				resp, err := client.Post(claimURL, "application/json", bytes.NewReader(body))
				if err != nil {
					atomic.AddInt64(&otherErrs, 1)
					continue
				}

				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()

				switch resp.StatusCode {
				case http.StatusOK, http.StatusCreated:
					atomic.AddInt64(&status200, 1)
				case http.StatusConflict:
					atomic.AddInt64(&status409, 1)
				case http.StatusGone:
					atomic.AddInt64(&status410, 1)
				default:
					atomic.AddInt64(&otherErrs, 1)
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	// 3. Inspect Redis Remaining Stock
	time.Sleep(1 * time.Second) // wait for async event propagation
	remainingStock := getRemainingStock(*claimHost, campaignID)

	fmt.Println("--------------------------------------------------")
	fmt.Println("📊 TEST RESULTS & CONCURRENCY ANALYSIS")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Elapsed Time:         %v\n", duration)
	fmt.Printf("Requests Per Second:  %.2f req/s\n", float64(totalReqs)/duration.Seconds())
	fmt.Printf("Success Claims (200): %d (Target: %d)\n", status200, *totalStock)
	fmt.Printf("Duplicate Blocked(409): %d\n", status409)
	fmt.Printf("Sold Out Blocked(410): %d\n", status410)
	fmt.Printf("Network/Other Errors: %d\n", otherErrs)
	fmt.Printf("Redis Remaining Stock: %d\n", remainingStock)
	fmt.Println("--------------------------------------------------")

	if status200 == int64(*totalStock) && remainingStock == 0 {
		fmt.Println("🏆 ZERO OVERSELLING VERIFIED: PASS! 🏆")
	} else {
		fmt.Printf("❌ INVENTORY MISMATCH! Expected %d successes, got %d. Remaining stock: %d\n",
			*totalStock, status200, remainingStock)
	}

	_ = voucherHost
}

func createTestCampaign(host string, stock int) (string, error) {
	reqBody, _ := json.Marshal(map[string]any{
		"title":           "Hackathon Flash Sale",
		"description":     "Load test flash sale campaign",
		"total_stock":     stock,
		"discount_amount": 50.0,
		"start_time":      time.Now().Add(-1 * time.Hour),
		"end_time":        time.Now().Add(24 * time.Hour),
	})

	resp, err := http.Post(host+"/api/v1/campaigns", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result CreateCampaignResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("api error: %s", result.Error)
	}

	return result.Data.ID, nil
}

func getRemainingStock(host, campaignID string) int {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/campaigns/%s/stock", host, campaignID))
	if err != nil {
		return -1
	}
	defer resp.Body.Close()

	var res struct {
		Data struct {
			RemainingStock int `json:"remaining_stock"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return res.Data.RemainingStock
}
