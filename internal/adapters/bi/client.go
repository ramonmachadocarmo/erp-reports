package bi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

type storagePlanLineDTO struct {
	ProductID   string  `json:"product_id"`
	ForecastQty float64 `json:"forecast_qty"`
	OnHandQty   float64 `json:"on_hand_qty"`
	OpenPOQty   float64 `json:"open_po_qty"`
	NeededQty   float64 `json:"needed_qty"`
}

func (c *Client) StoragePlan(ctx context.Context, coverageWeeks int, safetyPercent float64, lookbackWeeks int) ([]domain.StoragePlanLine, error) {
	q := url.Values{}
	q.Set("coverage_weeks", strconv.Itoa(coverageWeeks))
	q.Set("safety_percent", strconv.FormatFloat(safetyPercent, 'f', -1, 64))
	q.Set("lookback_weeks", strconv.Itoa(lookbackWeeks))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/storage-plan?"+q.Encode(), nil)
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
		return nil, fmt.Errorf("bi: %s", strings.TrimSpace(string(b)))
	}
	var out []storagePlanLineDTO
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	lines := make([]domain.StoragePlanLine, 0, len(out))
	for _, l := range out {
		lines = append(lines, domain.StoragePlanLine{
			ProductID: l.ProductID, ForecastQty: l.ForecastQty, OnHandQty: l.OnHandQty, OpenPOQty: l.OpenPOQty, NeededQty: l.NeededQty,
		})
	}
	return lines, nil
}
