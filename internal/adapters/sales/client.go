package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type orderItemDTO struct {
	ProductID string  `json:"product_id"`
	Quantity  float64 `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Subtotal  float64 `json:"subtotal"`
}

type orderDTO struct {
	ID          string         `json:"id"`
	CustomerID  string         `json:"customer_id"`
	Status      string         `json:"status"`
	TotalAmount float64        `json:"total_amount"`
	Items       []orderItemDTO `json:"items"`
	CreatedAt   time.Time      `json:"created_at"`
}

func (c *Client) Orders(ctx context.Context, from, to *time.Time) ([]domain.SalesOrder, error) {
	q := url.Values{}
	if from != nil {
		q.Set("from", from.UTC().Format(time.RFC3339))
	}
	if to != nil {
		q.Set("to", to.UTC().Format(time.RFC3339))
	}
	path := "/sales-orders"
	if qs := q.Encode(); qs != "" {
		path += "?" + qs
	}
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
		return nil, fmt.Errorf("sales: %s", strings.TrimSpace(string(b)))
	}
	var out []orderDTO
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	orders := make([]domain.SalesOrder, 0, len(out))
	for _, o := range out {
		items := make([]domain.OrderItem, 0, len(o.Items))
		for _, it := range o.Items {
			items = append(items, domain.OrderItem{ProductID: it.ProductID, Quantity: it.Quantity, UnitPrice: it.UnitPrice, Subtotal: it.Subtotal})
		}
		orders = append(orders, domain.SalesOrder{
			ID: o.ID, CustomerID: o.CustomerID, Status: o.Status, TotalAmount: o.TotalAmount, Items: items, CreatedAt: o.CreatedAt,
		})
	}
	return orders, nil
}
