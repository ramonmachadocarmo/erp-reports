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
	cashflow   domain.Cashflow
}

func New(catalog domain.Catalog, sales domain.SalesHistory, purchasing domain.Purchasing, directory domain.Directory, bi domain.BI, cashflow domain.Cashflow) *Service {
	return &Service{catalog: catalog, sales: sales, purchasing: purchasing, directory: directory, bi: bi, cashflow: cashflow}
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
			Code: a.Code, Name: a.Name, Items: items, Cost: a.Cost, SuggestedPrice: a.SuggestedPrice, MarginPercent: a.MarginPercent, Active: a.Active,
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

// CustomerRankingReport aggregates the same order data SalesReport lists one
// row per order, grouped by customer instead. CANCELLED orders are excluded
// since they never became real revenue.
func (s *Service) CustomerRankingReport(ctx context.Context, from, to *time.Time) ([]domain.CustomerRankingRow, error) {
	orders, err := s.sales.Orders(ctx, from, to)
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
	type agg struct {
		name  string
		count int
		total float64
	}
	byCustomer := make(map[string]*agg)
	var order []string
	for _, o := range orders {
		if o.Status == "CANCELLED" {
			continue
		}
		a, ok := byCustomer[o.CustomerID]
		if !ok {
			name := nameLookup[o.CustomerID]
			if name == "" {
				name = o.CustomerID
			}
			a = &agg{name: name}
			byCustomer[o.CustomerID] = a
			order = append(order, o.CustomerID)
		}
		a.count++
		a.total += o.TotalAmount
	}
	out := make([]domain.CustomerRankingRow, 0, len(byCustomer))
	for _, id := range order {
		a := byCustomer[id]
		var avg float64
		if a.count > 0 {
			avg = a.total / float64(a.count)
		}
		out = append(out, domain.CustomerRankingRow{CustomerName: a.name, OrderCount: a.count, TotalAmount: a.total, AverageTicket: avg})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalAmount > out[j].TotalAmount })
	return out, nil
}

// ProductSalesReport aggregates the same order items ItemSummary already
// walks, grouped by product instead of concatenated into one text column.
// CANCELLED orders are excluded, same as CustomerRankingReport.
func (s *Service) ProductSalesReport(ctx context.Context, from, to *time.Time) ([]domain.ProductSalesRow, error) {
	orders, err := s.sales.Orders(ctx, from, to)
	if err != nil {
		return nil, err
	}
	products, err := s.productLookup(ctx)
	if err != nil {
		return nil, err
	}
	type agg struct {
		qty    float64
		amount float64
		orders map[string]struct{}
	}
	byProduct := make(map[string]*agg)
	var order []string
	for _, o := range orders {
		if o.Status == "CANCELLED" {
			continue
		}
		for _, it := range o.Items {
			a, ok := byProduct[it.ProductID]
			if !ok {
				a = &agg{orders: map[string]struct{}{}}
				byProduct[it.ProductID] = a
				order = append(order, it.ProductID)
			}
			a.qty += it.Quantity
			a.amount += it.Subtotal
			a.orders[o.ID] = struct{}{}
		}
	}
	out := make([]domain.ProductSalesRow, 0, len(byProduct))
	for _, id := range order {
		a := byProduct[id]
		p := products[id]
		out = append(out, domain.ProductSalesRow{
			SKU: p.SKU, ProductName: p.Name, UoM: uom(p), Quantity: a.qty, TotalAmount: a.amount, OrderCount: len(a.orders),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalAmount > out[j].TotalAmount })
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

// LossesReport pushes date/product/warehouse filtering down to stock-service's
// GET /movements (subtype=LOSS) rather than filtering in memory, since
// movements are a much larger, unbounded dataset than orders.
func (s *Service) LossesReport(ctx context.Context, from, to *time.Time, productID, warehouseID string) ([]domain.LossReportRow, error) {
	movements, err := s.catalog.Movements(ctx, domain.MovementFilter{
		ProductID: productID, WarehouseID: warehouseID, Subtype: "LOSS", From: from, To: to,
	})
	if err != nil {
		return nil, err
	}
	products, err := s.productLookup(ctx)
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
	out := make([]domain.LossReportRow, 0, len(movements))
	for _, m := range movements {
		p := products[m.ProductID]
		w := whLookup[m.WarehouseID]
		out = append(out, domain.LossReportRow{
			SKU: p.SKU, ProductName: p.Name, WarehouseCode: w.Code, WarehouseName: w.Name,
			UoM: uom(p), Quantity: m.Quantity, CreatedAt: m.CreatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// CashflowReport filters cashflow-service's entries (schedule + manual) in
// memory, the same shortcut PurchasesReport takes: GET /entries has no query
// params today and cashflow-service's own screen already fetches everything,
// so the dataset is small and bounded enough not to need a new endpoint.
func (s *Service) CashflowReport(ctx context.Context, from, to *time.Time, direction, status string, onlyOverdue bool) ([]domain.CashflowReportRow, error) {
	entries, err := s.cashflow.Entries(ctx)
	if err != nil {
		return nil, err
	}
	customers, err := s.directory.Customers(ctx)
	if err != nil {
		return nil, err
	}
	suppliers, err := s.directory.Suppliers(ctx)
	if err != nil {
		return nil, err
	}
	custLookup := make(map[string]string, len(customers))
	for _, c := range customers {
		custLookup[c.ID] = c.Name
	}
	supLookup := make(map[string]string, len(suppliers))
	for _, sup := range suppliers {
		supLookup[sup.ID] = sup.Name
	}
	now := time.Now()
	out := make([]domain.CashflowReportRow, 0, len(entries))
	for _, e := range entries {
		if from != nil && e.DueDate.Before(*from) {
			continue
		}
		if to != nil && e.DueDate.After(*to) {
			continue
		}
		if direction != "" && e.Direction != direction {
			continue
		}
		if status != "" && e.Status != status {
			continue
		}
		overdue := e.Status == "PENDING" && e.DueDate.Before(now)
		if onlyOverdue && !overdue {
			continue
		}
		var partyName string
		switch e.ReferenceType {
		case "SALE":
			partyName = custLookup[e.PartyID]
		case "PURCHASE":
			partyName = supLookup[e.PartyID]
		}
		out = append(out, domain.CashflowReportRow{
			DueDate: e.DueDate, Direction: e.Direction, Amount: e.Amount, Status: e.Status, Overdue: overdue,
			PartyName: partyName, PaymentMethodName: e.PaymentMethodName, PaymentTermName: e.PaymentTermName,
			InstallmentNo: e.InstallmentNo, InstallmentsTotal: e.InstallmentsTotal,
			ReferenceType: e.ReferenceType, Description: e.Description,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DueDate.Before(out[j].DueDate) })
	return out, nil
}

// CashflowTimelineReport reuses the same cashflow-service entries as
// CashflowReport, grouped by due date and split realizado (CONFIRMED) ×
// projetado (PENDING) instead of listed one row per entry.
func (s *Service) CashflowTimelineReport(ctx context.Context, from, to *time.Time) ([]domain.CashflowTimelineRow, error) {
	entries, err := s.cashflow.Entries(ctx)
	if err != nil {
		return nil, err
	}
	type agg struct {
		realizedIn, realizedOut   float64
		projectedIn, projectedOut float64
	}
	byDay := make(map[string]*agg)
	var dates []string
	for _, e := range entries {
		if from != nil && e.DueDate.Before(*from) {
			continue
		}
		if to != nil && e.DueDate.After(*to) {
			continue
		}
		d := e.DueDate.UTC().Format("2006-01-02")
		a, ok := byDay[d]
		if !ok {
			a = &agg{}
			byDay[d] = a
			dates = append(dates, d)
		}
		switch {
		case e.Status == "CONFIRMED" && e.Direction == "IN":
			a.realizedIn += e.Amount
		case e.Status == "CONFIRMED":
			a.realizedOut += e.Amount
		case e.Direction == "IN":
			a.projectedIn += e.Amount
		default:
			a.projectedOut += e.Amount
		}
	}
	sort.Strings(dates)
	var balance float64
	out := make([]domain.CashflowTimelineRow, 0, len(dates))
	for _, d := range dates {
		a := byDay[d]
		realizedNet := a.realizedIn - a.realizedOut
		projectedNet := a.projectedIn - a.projectedOut
		balance += realizedNet + projectedNet
		out = append(out, domain.CashflowTimelineRow{
			Date: d, RealizedInflow: a.realizedIn, RealizedOutflow: a.realizedOut, RealizedNet: realizedNet,
			ProjectedInflow: a.projectedIn, ProjectedOutflow: a.projectedOut, ProjectedNet: projectedNet,
			Balance: balance,
		})
	}
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
