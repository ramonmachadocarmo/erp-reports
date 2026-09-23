package cashflow

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

type entryDTO struct {
	DueDate           time.Time `json:"due_date"`
	Amount            float64   `json:"amount"`
	Direction         string    `json:"direction"`
	Status            string    `json:"status"`
	PaymentMethodName string    `json:"payment_method_name"`
	PaymentTermName   string    `json:"payment_term_name"`
	InstallmentNo     int       `json:"installment_no"`
	InstallmentsTotal int       `json:"installments_total"`
	ReferenceType     string    `json:"reference_type"`
	PartyID           string    `json:"party_id"`
	Description       string    `json:"description"`
}

func (c *Client) Entries(ctx context.Context) ([]domain.CashflowEntry, error) {
	var out []entryDTO
	if err := c.get(ctx, "/entries", &out); err != nil {
		return nil, err
	}
	entries := make([]domain.CashflowEntry, 0, len(out))
	for _, e := range out {
		entries = append(entries, domain.CashflowEntry{
			DueDate: e.DueDate, Amount: e.Amount, Direction: e.Direction, Status: e.Status,
			PaymentMethodName: e.PaymentMethodName, PaymentTermName: e.PaymentTermName,
			InstallmentNo: e.InstallmentNo, InstallmentsTotal: e.InstallmentsTotal,
			ReferenceType: e.ReferenceType, PartyID: e.PartyID, Description: e.Description,
		})
	}
	return entries, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	if tok, ok := ctx.Value(AuthHeaderKey).(string); ok && tok != "" {
		req.Header.Set("Authorization", tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cashflow: %s", strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
