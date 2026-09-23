package domain

import (
	"context"
	"time"
)

// --- report row shapes ---

// KitReportItem is one BOM line of a kit report, priced in the item's own
// sale/stock UoM (not its purchase UoM) so it lines up with Quantity. UnitCost
// is the purchase price converted into that UoM; LineCost = Quantity ×
// UnitCost; ProportionalSalePrice is this line's share of the kit's own
// SuggestedPrice, proportional to its share of the kit's total Cost — i.e.
// "how much of what the kit sells for is this ingredient."
type KitReportItem struct {
	ProductSKU            string  `json:"product_sku"`
	ProductName           string  `json:"product_name"`
	Quantity              float64 `json:"quantity"`
	UoM                   string  `json:"uom"`
	Role                  string  `json:"role"`
	UnitCost              float64 `json:"unit_cost"`
	LineCost              float64 `json:"line_cost"`
	ProportionalSalePrice float64 `json:"proportional_sale_price"`
}

type KitReportRow struct {
	Code           string          `json:"code"`
	Name           string          `json:"name"`
	Items          []KitReportItem `json:"items"`
	Cost           float64         `json:"cost"`
	SuggestedPrice float64         `json:"suggested_price"`
	MarginPercent  float64         `json:"margin_percent"`
	Active         bool            `json:"active"`
}

type StockReportRow struct {
	SKU               string  `json:"sku"`
	ProductName       string  `json:"product_name"`
	WarehouseCode     string  `json:"warehouse_code"`
	WarehouseName     string  `json:"warehouse_name"`
	UoM               string  `json:"uom"`
	QuantityAvailable float64 `json:"quantity_available"`
	QuantityReserved  float64 `json:"quantity_reserved"`
}

type SalesReportRow struct {
	OrderID      string    `json:"order_id"`
	CreatedAt    time.Time `json:"created_at"`
	CustomerName string    `json:"customer_name"`
	Status       string    `json:"status"`
	ItemSummary  string    `json:"item_summary"`
	TotalAmount  float64   `json:"total_amount"`
}

// CustomerRankingRow aggregates SalesOrder by customer — see
// Service.CustomerRankingReport for why CANCELLED orders are excluded.
type CustomerRankingRow struct {
	CustomerName  string  `json:"customer_name"`
	OrderCount    int     `json:"order_count"`
	TotalAmount   float64 `json:"total_amount"`
	AverageTicket float64 `json:"average_ticket"`
}

// ProductSalesRow aggregates sold order lines by product — same order data
// SalesReport/CustomerRankingReport already fetch, grouped by product
// instead. Amount comes from each line's own Subtotal (qty × unit price at
// sale time), not the product's current sale price, so it reflects what was
// actually invoiced. A kit line is counted as the kit's own product, not
// expanded into its components — same as ItemSummary elsewhere.
type ProductSalesRow struct {
	SKU         string  `json:"sku"`
	ProductName string  `json:"product_name"`
	UoM         string  `json:"uom"`
	Quantity    float64 `json:"quantity"`
	TotalAmount float64 `json:"total_amount"`
	OrderCount  int     `json:"order_count"`
}

type PurchaseReportRow struct {
	OrderID      string    `json:"order_id"`
	CreatedAt    time.Time `json:"created_at"`
	SupplierName string    `json:"supplier_name"`
	Status       string    `json:"status"`
	ItemSummary  string    `json:"item_summary"`
	TotalAmount  float64   `json:"total_amount"`
}

type LossReportRow struct {
	SKU           string    `json:"sku"`
	ProductName   string    `json:"product_name"`
	WarehouseCode string    `json:"warehouse_code"`
	WarehouseName string    `json:"warehouse_name"`
	UoM           string    `json:"uom"`
	Quantity      float64   `json:"quantity"`
	CreatedAt     time.Time `json:"created_at"`
}

// CashflowReportRow is one contas-a-pagar/receber row: a scheduled or manual
// cashflow-service entry, joined with the customer/supplier name behind its
// SALE/PURCHASE reference (MANUAL entries have no party). Overdue is derived
// here (Status == PENDING and DueDate has passed) rather than stored, since
// cashflow-service itself never marks an entry late — it only flips PENDING
// to CONFIRMED on payment.
type CashflowReportRow struct {
	DueDate           time.Time `json:"due_date"`
	Direction         string    `json:"direction"`
	Amount            float64   `json:"amount"`
	Status            string    `json:"status"`
	Overdue           bool      `json:"overdue"`
	PartyName         string    `json:"party_name"`
	PaymentMethodName string    `json:"payment_method_name"`
	PaymentTermName   string    `json:"payment_term_name"`
	InstallmentNo     int       `json:"installment_no"`
	InstallmentsTotal int       `json:"installments_total"`
	ReferenceType     string    `json:"reference_type"`
	Description       string    `json:"description"`
}

// CashflowTimelineRow is one day of the realizado×projetado cash flow: the
// same entries behind CashflowReportRow, grouped by due date and split by
// Status (CONFIRMED = realizado, PENDING = projetado) instead of listed one
// row per entry. Balance is the running cumulative net (realized+projected)
// up to and including this day — mirrors cashflow-service's own
// domain.Summarize, just keeping realized and projected apart instead of
// merging them into one Net/Balance.
type CashflowTimelineRow struct {
	Date             string  `json:"date"`
	RealizedInflow   float64 `json:"realized_inflow"`
	RealizedOutflow  float64 `json:"realized_outflow"`
	RealizedNet      float64 `json:"realized_net"`
	ProjectedInflow  float64 `json:"projected_inflow"`
	ProjectedOutflow float64 `json:"projected_outflow"`
	ProjectedNet     float64 `json:"projected_net"`
	Balance          float64 `json:"balance"`
}

type ForecastReportRow struct {
	SKU         string  `json:"sku"`
	ProductName string  `json:"product_name"`
	UoM         string  `json:"uom"`
	ForecastQty float64 `json:"forecast_qty"`
	OnHandQty   float64 `json:"on_hand_qty"`
	OpenPOQty   float64 `json:"open_po_qty"`
	NeededQty   float64 `json:"needed_qty"`
}

// --- peer-service data shapes ---

type Product struct {
	ID            string
	SKU           string
	Name          string
	Kind          string
	SaleUoM       string
	StockUoM      string
	PurchasePrice float64
	PurchaseUoM   string
	Conversions   []UoMConversion
}

// StockUnitOfMeasure is the unit item quantities are expressed in.
func (p Product) StockUnitOfMeasure() string {
	if p.StockUoM != "" {
		return p.StockUoM
	}
	return p.SaleUoM
}

type UoMConversion struct {
	FromUoM string
	ToUoM   string
	Factor  float64
}

// ConvertQty converts a quantity between two units using the registered
// conversions (checked in either direction). ok is false when from/to
// genuinely differ and no matching factor is registered.
func ConvertQty(qty float64, from, to string, convs []UoMConversion) (float64, bool) {
	if from == "" || to == "" || from == to {
		return qty, true
	}
	for _, c := range convs {
		if c.Factor <= 0 {
			continue
		}
		if c.FromUoM == from && c.ToUoM == to {
			return qty * c.Factor, true
		}
		if c.FromUoM == to && c.ToUoM == from {
			return qty / c.Factor, true
		}
	}
	return 0, false
}

type Balance struct {
	ProductID         string
	WarehouseID       string
	QuantityAvailable float64
	QuantityReserved  float64
}

type Warehouse struct {
	ID   string
	Code string
	Name string
}

type AssemblyItem struct {
	ProductID string
	Quantity  float64
	Role      string
}

type Assembly struct {
	Code           string
	Name           string
	Items          []AssemblyItem
	Cost           float64
	SuggestedPrice float64
	MarginPercent  float64
	Active         bool
}

type OrderItem struct {
	ProductID string
	Quantity  float64
	UnitPrice float64
	Subtotal  float64
}

type SalesOrder struct {
	ID          string
	CustomerID  string
	Status      string
	TotalAmount float64
	Items       []OrderItem
	CreatedAt   time.Time
}

type PurchaseOrder struct {
	ID          string
	SupplierID  string
	Status      string
	TotalAmount float64
	Items       []OrderItem
	CreatedAt   time.Time
}

type Person struct {
	ID   string
	Name string
}

type Movement struct {
	ProductID   string
	WarehouseID string
	Quantity    float64
	CreatedAt   time.Time
}

// MovementFilter is pushed down to stock-service's GET /movements so the
// dataset stays bounded — see stock-service's own domain.MovementFilter.
type MovementFilter struct {
	ProductID   string
	WarehouseID string
	Subtype     string
	From        *time.Time
	To          *time.Time
}

type CashflowEntry struct {
	DueDate           time.Time
	Amount            float64
	Direction         string
	Status            string
	PaymentMethodName string
	PaymentTermName   string
	InstallmentNo     int
	InstallmentsTotal int
	ReferenceType     string
	PartyID           string
	Description       string
}

type StoragePlanLine struct {
	ProductID   string
	ForecastQty float64
	OnHandQty   float64
	OpenPOQty   float64
	NeededQty   float64
}

// --- peer-service ports ---
//
// Every method here calls another service's existing HTTP API — reports-service
// holds no direct database connection of its own, and never touches another
// service's Postgres pool. That keeps report generation, however heavy, from
// ever contending for connections or locks with normal transactional traffic.

type Catalog interface {
	Products(ctx context.Context) ([]Product, error)
	Balances(ctx context.Context) ([]Balance, error)
	Warehouses(ctx context.Context) ([]Warehouse, error)
	Assemblies(ctx context.Context) ([]Assembly, error)
	Movements(ctx context.Context, f MovementFilter) ([]Movement, error)
}

type SalesHistory interface {
	Orders(ctx context.Context, from, to *time.Time) ([]SalesOrder, error)
}

type Purchasing interface {
	Orders(ctx context.Context) ([]PurchaseOrder, error)
}

type Directory interface {
	Customers(ctx context.Context) ([]Person, error)
	Suppliers(ctx context.Context) ([]Person, error)
}

type BI interface {
	StoragePlan(ctx context.Context, coverageWeeks int, safetyPercent float64, lookbackWeeks int) ([]StoragePlanLine, error)
}

type Cashflow interface {
	Entries(ctx context.Context) ([]CashflowEntry, error)
}
