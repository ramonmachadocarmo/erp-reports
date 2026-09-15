package application

import (
	"testing"

	"erp/services/reports-service/internal/domain"
)

func TestItemCostPerSaleUnit_ConvertsFromPurchaseUoM(t *testing.T) {
	// $110/CX, 1 CX = 20kg -> $5.50/kg.
	p := domain.Product{
		PurchasePrice: 110,
		PurchaseUoM:   "CX",
		StockUoM:      "KG",
		Conversions:   []domain.UoMConversion{{FromUoM: "CX", ToUoM: "KG", Factor: 20}},
	}
	got, ok := itemCostPerSaleUnit(p)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != 5.5 {
		t.Errorf("got %v, want 5.5", got)
	}
}

func TestItemCostPerSaleUnit_SameUoMPassesThrough(t *testing.T) {
	p := domain.Product{PurchasePrice: 12, PurchaseUoM: "UN", StockUoM: "UN"}
	got, ok := itemCostPerSaleUnit(p)
	if !ok || got != 12 {
		t.Errorf("got (%v, %v), want (12, true)", got, ok)
	}
}

func TestItemCostPerSaleUnit_NoConversionAvailable(t *testing.T) {
	p := domain.Product{PurchasePrice: 12, PurchaseUoM: "CX", StockUoM: "KG"}
	_, ok := itemCostPerSaleUnit(p)
	if ok {
		t.Fatal("expected ok=false when there's no registered conversion")
	}
}
