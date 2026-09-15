package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"erp/services/reports-service/internal/domain"
)

type ctxKey string

const AuthHeaderKey ctxKey = "authorization"

type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"),
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

type personDTO struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	CompanyName string `json:"company_name"`
}

func personName(p personDTO) string {
	if p.Kind == "PJ" && p.CompanyName != "" {
		return p.CompanyName
	}
	return p.Name
}

func (c *Client) Customers(ctx context.Context) ([]domain.Person, error) {
	return c.people(ctx, "/customers")
}

func (c *Client) Suppliers(ctx context.Context) ([]domain.Person, error) {
	return c.people(ctx, "/suppliers")
}

func (c *Client) people(ctx context.Context, path string) ([]domain.Person, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	if tok, ok := ctx.Value(AuthHeaderKey).(string); ok && tok != "" {
		req.Header.Set("Authorization", tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("config: %s", strings.TrimSpace(string(b)))
	}
	var out []personDTO
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	people := make([]domain.Person, 0, len(out))
	for _, p := range out {
		people = append(people, domain.Person{ID: p.ID, Name: personName(p)})
	}
	return people, nil
}
