package stock

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

type uomConversionDTO struct {
	FromUoM string  `json:"from_uom"`
	ToUoM   string  `json:"to_uom"`
	Factor  float64 `json:"factor"`
}

type productDTO struct {
	ID             string             `json:"id"`
	SKU            string             `json:"sku"`
	Name           string             `json:"name"`
	Kind           string             `json:"kind"`
	SaleUoM        string             `json:"sale_uom"`
	StockUoM       string             `json:"stock_uom"`
	PurchasePrice  float64            `json:"purchase_price"`
	PurchaseUoM    string             `json:"purchase_uom"`
	UoMConversions []uomConversionDTO `json:"uom_conversions"`
}

type balanceDTO struct {
	ProductID         string  `json:"product_id"`
	WarehouseID       string  `json:"warehouse_id"`
	QuantityAvailable float64 `json:"quantity_available"`
	QuantityReserved  float64 `json:"quantity_reserved"`
}

type warehouseDTO struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type assemblyItemDTO struct {
	ProductID string  `json:"product_id"`
	Quantity  float64 `json:"quantity"`
	Role      string  `json:"role"`
}

type assemblyDTO struct {
	Code           string            `json:"code"`
	Name           string            `json:"name"`
	Items          []assemblyItemDTO `json:"items"`
	Cost           float64           `json:"cost"`
	SuggestedPrice float64           `json:"suggested_price"`
	MarginPercent  float64           `json:"margin_percent"`
	Active         bool              `json:"active"`
}

func (c *Client) Products(ctx context.Context) ([]domain.Product, error) {
	var out []productDTO
	if err := c.get(ctx, "/products", &out); err != nil {
		return nil, err
	}
	products := make([]domain.Product, 0, len(out))
	for _, p := range out {
		convs := make([]domain.UoMConversion, 0, len(p.UoMConversions))
		for _, c := range p.UoMConversions {
			convs = append(convs, domain.UoMConversion{FromUoM: c.FromUoM, ToUoM: c.ToUoM, Factor: c.Factor})
		}
		products = append(products, domain.Product{
			ID: p.ID, SKU: p.SKU, Name: p.Name, Kind: p.Kind, SaleUoM: p.SaleUoM, StockUoM: p.StockUoM,
			PurchasePrice: p.PurchasePrice, PurchaseUoM: p.PurchaseUoM, Conversions: convs,
		})
	}
	return products, nil
}

func (c *Client) Balances(ctx context.Context) ([]domain.Balance, error) {
	var out []balanceDTO
	if err := c.get(ctx, "/balances", &out); err != nil {
		return nil, err
	}
	balances := make([]domain.Balance, 0, len(out))
	for _, b := range out {
		balances = append(balances, domain.Balance{ProductID: b.ProductID, WarehouseID: b.WarehouseID, QuantityAvailable: b.QuantityAvailable, QuantityReserved: b.QuantityReserved})
	}
	return balances, nil
}

func (c *Client) Warehouses(ctx context.Context) ([]domain.Warehouse, error) {
	var out []warehouseDTO
	if err := c.get(ctx, "/warehouses", &out); err != nil {
		return nil, err
	}
	warehouses := make([]domain.Warehouse, 0, len(out))
	for _, w := range out {
		warehouses = append(warehouses, domain.Warehouse{ID: w.ID, Code: w.Code, Name: w.Name})
	}
	return warehouses, nil
}

func (c *Client) Assemblies(ctx context.Context) ([]domain.Assembly, error) {
	var out []assemblyDTO
	if err := c.get(ctx, "/assemblies", &out); err != nil {
		return nil, err
	}
	assemblies := make([]domain.Assembly, 0, len(out))
	for _, a := range out {
		items := make([]domain.AssemblyItem, 0, len(a.Items))
		for _, it := range a.Items {
			items = append(items, domain.AssemblyItem{ProductID: it.ProductID, Quantity: it.Quantity, Role: it.Role})
		}
		assemblies = append(assemblies, domain.Assembly{
			Code: a.Code, Name: a.Name, Items: items, Cost: a.Cost, SuggestedPrice: a.SuggestedPrice, MarginPercent: a.MarginPercent, Active: a.Active,
		})
	}
	return assemblies, nil
}

type movementDTO struct {
	ProductID   string    `json:"product_id"`
	WarehouseID string    `json:"warehouse_id"`
	Quantity    float64   `json:"quantity"`
	CreatedAt   time.Time `json:"created_at"`
}

func (c *Client) Movements(ctx context.Context, f domain.MovementFilter) ([]domain.Movement, error) {
	q := url.Values{}
	if f.ProductID != "" {
		q.Set("product_id", f.ProductID)
	}
	if f.WarehouseID != "" {
		q.Set("warehouse_id", f.WarehouseID)
	}
	if f.Subtype != "" {
		q.Set("subtype", f.Subtype)
	}
	if f.From != nil {
		q.Set("from", f.From.Format(time.RFC3339))
	}
	if f.To != nil {
		q.Set("to", f.To.Format(time.RFC3339))
	}
	var out []movementDTO
	if err := c.get(ctx, "/movements?"+q.Encode(), &out); err != nil {
		return nil, err
	}
	movements := make([]domain.Movement, 0, len(out))
	for _, m := range out {
		movements = append(movements, domain.Movement{ProductID: m.ProductID, WarehouseID: m.WarehouseID, Quantity: m.Quantity, CreatedAt: m.CreatedAt})
	}
	return movements, nil
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
		return fmt.Errorf("stock: %s", strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
