// Command token generates a development JWT for testing protected endpoints.
//
// Usage:
//
//	go run ./scripts/token -sub cust-demo -phone 08123456789
//
// The secret is read from JWT_SECRET (defaults to the hackathon dev secret).
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"voucher-flash-sale/pkg/jwt"
)

const defaultSecret = "salt-hackathon-jwt-secret-key-32bytes"

func main() {
	sub := flag.String("sub", "cust-demo", "token subject (customer id)")
	phone := flag.String("phone", "08123456789", "customer phone number")
	ttl := flag.Duration("ttl", time.Hour, "token validity duration")
	flag.Parse()

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = defaultSecret
	}

	pair, err := jwt.GenerateTokens(*sub, *phone, true, secret, *ttl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate token: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(pair.AccessToken)
}
