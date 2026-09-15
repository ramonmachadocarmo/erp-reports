package application

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"erp/services/reports-service/internal/domain"
)

type Service struct {
	catalog    domain.Catalog
	sales      domain.SalesHistory
	purchasing domain.Purchasing
	directory  domain.Directory
	bi         domain.BI
}

func New(catalog domain.Catalog, sales domain.SalesHistory, purchasing domain.Purchasing, directory domain.Directory, bi domain.BI) *Service {
	return &Service{catalog: catalog, sales: sales, purchasing: purchasing, directory: directory, bi: bi}
}

func (s *Service) productLookup(ctx context.Context) (map[string]domain.Product, error) {
	products, err := s.catalog.Products(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]domain.Product, len(products))
	for _, p := range products {
		m[p.ID] = p
	}
	return m, nil
}

func uom(p domain.Product) string {
	if p.StockUoM != "" {
		return p.StockUoM
	}
	return p.SaleUoM
}

func trimQty(q float64) string {
	s := strconv.FormatFloat(q, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

func itemSummary(items []domain.OrderItem, products map[string]domain.Product) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		name := products[it.ProductID].Name
		if name == "" {
			name = it.ProductID
		}
		parts = append(parts, fmt.Sprintf("%s × %s", name, trimQty(it.Quantity)))
	}
	return strings.Join(parts, ", ")
}

// itemCostPerSaleUnit converts a product's purchase price into a price per
// unit of its own sale/stock UoM (e.g. $110/CX, 1 CX = 20kg -> $5.50/kg) —
// mirrors bi-service's and the stock MFE's Montagem-form calc so all three
// agree on what an item "costs" per kg/un/etc.
func itemCostPerSaleUnit(p domain.Product) (float64, bool) {
	saleUoM := p.StockUnitOfMeasure()
	if p.PurchaseUoM == "" || saleUoM == "" || p.PurchaseUoM == saleUoM {
		return p.PurchasePrice, true
	}
	stockPerPurchaseUnit, ok := domain.ConvertQty(1, p.PurchaseUoM, saleUoM, p.Conversions)
	if !ok || stockPerPurchaseUnit <= 0 {
		return 0, false
	}
	return p.PurchasePrice / stockPerPurchaseUnit, true
}

func (s *Service) KitsReport(ctx context.Context) ([]domain.KitReportRow, error) {
	assemblies, err := s.catalog.Assemblies(ctx)
	if err != nil {
		return nil, err
	}
	products, err := s.productLookup(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.KitReportRow, 0, len(assemblies))
	for _, a := range assemblies {
		items := make([]domain.KitReportItem, 0, len(a.Items))
		for _, it := range a.Items {
			p := products[it.ProductID]
			unitCost, ok := itemCostPerSaleUnit(p)
			lineCost := 0.0
			if ok {
				lineCost = unitCost * it.Quantity
			}
			items = append(items, domain.KitReportItem{
				ProductSKU: p.SKU, ProductName: p.Name, Quantity: it.Quantity, UoM: uom(p), Role: it.Role,
				UnitCost: unitCost, LineCost: lineCost,
			})
		}
		if a.Cost > 0 {
			for i := range items {
				items[i].ProportionalSalePrice = items[i].LineCost / a.Cost * a.SuggestedPrice
			}
		}
		out = append(out, domain.KitReportRow{
			Code: a.Code, Name: a.Name, Items: items, Cost: a.Cost, SuggestedPrice: a.SuggestedPrice, MarginPercent: a.MarginPercent,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (s *Service) StockReport(ctx context.Context, warehouseID string) ([]domain.StockReportRow, error) {
	products, err := s.productLookup(ctx)
	if err != nil {
		return nil, err
	}
	balances, err := s.catalog.Balances(ctx)
	if err != nil {
		return nil, err
	}
	warehouses, err := s.catalog.Warehouses(ctx)
	if err != nil {
		return nil, err
	}
	whLookup := make(map[string]domain.Warehouse, len(warehouses))
	for _, w := range warehouses {
		whLookup[w.ID] = w
	}
	out := make([]domain.StockReportRow, 0, len(balances))
	for _, b := range balances {
		if warehouseID != "" && b.WarehouseID != warehouseID {
			continue
		}
		p := products[b.ProductID]
		w := whLookup[b.WarehouseID]
		out = append(out, domain.StockReportRow{
			SKU: p.SKU, ProductName: p.Name, WarehouseCode: w.Code, WarehouseName: w.Name,
			UoM: uom(p), QuantityAvailable: b.QuantityAvailable, QuantityReserved: b.QuantityReserved,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SKU != out[j].SKU {
			return out[i].SKU < out[j].SKU
		}
		return out[i].WarehouseCode < out[j].WarehouseCode
	})
	return out, nil
}

func (s *Service) SalesReport(ctx context.Context, from, to *time.Time) ([]domain.SalesReportRow, error) {
	orders, err := s.sales.Orders(ctx, from, to)
	if err != nil {
		return nil, err
	}
	products, err := s.productLookup(ctx)
	if err != nil {
		return nil, err
	}
	customers, err := s.directory.Customers(ctx)
	if err != nil {
		return nil, err
	}
	nameLookup := make(map[string]string, len(customers))
	for _, c := range customers {
		nameLookup[c.ID] = c.Name
	}
	out := make([]domain.SalesReportRow, 0, len(orders))
	for _, o := range orders {
		name := nameLookup[o.CustomerID]
		if name == "" {
			name = o.CustomerID
		}
		out = append(out, domain.SalesReportRow{
			OrderID: o.ID, CreatedAt: o.CreatedAt, CustomerName: name, Status: o.Status,
			ItemSummary: itemSummary(o.Items, products), TotalAmount: o.TotalAmount,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// PurchasesReport filters by date in memory since purchasing-service's list
// endpoint has no date params — cheaper than adding a new endpoint there for
// what's normally a small, bounded dataset, and it still never touches
// purchasing-service's database directly.
func (s *Service) PurchasesReport(ctx context.Context, from, to *time.Time) ([]domain.PurchaseReportRow, error) {
	orders, err := s.purchasing.Orders(ctx)
	if err != nil {
		return nil, err
	}
	products, err := s.productLookup(ctx)
	if err != nil {
		return nil, err
	}
	suppliers, err := s.directory.Suppliers(ctx)
	if err != nil {
		return nil, err
	}
	nameLookup := make(map[string]string, len(suppliers))
	for _, sup := range suppliers {
		nameLookup[sup.ID] = sup.Name
	}
	out := make([]domain.PurchaseReportRow, 0, len(orders))
	for _, o := range orders {
		if from != nil && o.CreatedAt.Before(*from) {
			continue
		}
		if to != nil && o.CreatedAt.After(*to) {
			continue
		}
		name := nameLookup[o.SupplierID]
		if name == "" {
			name = o.SupplierID
		}
		out = append(out, domain.PurchaseReportRow{
			OrderID: o.ID, CreatedAt: o.CreatedAt, SupplierName: name, Status: o.Status,
			ItemSummary: itemSummary(o.Items, products), TotalAmount: o.TotalAmount,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *Service) ForecastReport(ctx context.Context, coverageWeeks int, safetyPercent float64, lookbackWeeks int) ([]domain.ForecastReportRow, error) {
	lines, err := s.bi.StoragePlan(ctx, coverageWeeks, safetyPercent, lookbackWeeks)
	if err != nil {
		return nil, err
	}
	products, err := s.productLookup(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ForecastReportRow, 0, len(lines))
	for _, l := range lines {
		p := products[l.ProductID]
		out = append(out, domain.ForecastReportRow{
			SKU: p.SKU, ProductName: p.Name, UoM: uom(p),
			ForecastQty: l.ForecastQty, OnHandQty: l.OnHandQty, OpenPOQty: l.OpenPOQty, NeededQty: l.NeededQty,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SKU < out[j].SKU })
	return out, nil
}
