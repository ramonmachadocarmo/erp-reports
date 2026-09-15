package purchasing

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

type orderItemDTO struct {
	ProductID string  `json:"product_id"`
	Quantity  float64 `json:"quantity"`
}

type orderDTO struct {
	ID          string         `json:"id"`
	SupplierID  string         `json:"supplier_id"`
	Status      string         `json:"status"`
	TotalAmount float64        `json:"total_amount"`
	Items       []orderItemDTO `json:"items"`
	CreatedAt   time.Time      `json:"created_at"`
}

// Orders fetches the full purchase-order list — purchasing-service has no
// date-filtered endpoint (unlike sales-service), so callers filter in memory.
func (c *Client) Orders(ctx context.Context) ([]domain.PurchaseOrder, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/purchase-orders", nil)
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
		return nil, fmt.Errorf("purchasing: %s", strings.TrimSpace(string(b)))
	}
	var out []orderDTO
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	orders := make([]domain.PurchaseOrder, 0, len(out))
	for _, o := range out {
		items := make([]domain.OrderItem, 0, len(o.Items))
		for _, it := range o.Items {
			items = append(items, domain.OrderItem{ProductID: it.ProductID, Quantity: it.Quantity})
		}
		orders = append(orders, domain.PurchaseOrder{
			ID: o.ID, SupplierID: o.SupplierID, Status: o.Status, TotalAmount: o.TotalAmount, Items: items, CreatedAt: o.CreatedAt,
		})
	}
	return orders, nil
}
