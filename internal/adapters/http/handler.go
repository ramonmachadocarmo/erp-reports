package httpadapter

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"erp/pkg/httpserver"
	biclient "erp/services/reports-service/internal/adapters/bi"
	cashflowclient "erp/services/reports-service/internal/adapters/cashflow"
	configclient "erp/services/reports-service/internal/adapters/config"
	purchasingclient "erp/services/reports-service/internal/adapters/purchasing"
	salesclient "erp/services/reports-service/internal/adapters/sales"
	stockclient "erp/services/reports-service/internal/adapters/stock"
	"erp/services/reports-service/internal/application"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *application.Service
}

func New(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.Engine, jwt gin.HandlerFunc) {
	api := r.Group("/", jwt)
	api.GET("/kits", h.kits)
	api.GET("/stock", h.stock)
	api.GET("/sales", h.sales)
	api.GET("/customer-ranking", h.customerRanking)
	api.GET("/customer-ranking/:id", h.customerDetail)
	api.GET("/product-sales", h.productSales)
	api.GET("/purchases", h.purchases)
	api.GET("/losses", h.losses)
	api.GET("/cashflow", h.cashflow)
	api.GET("/cashflow-timeline", h.cashflowTimeline)
	api.GET("/forecast", h.forecast)
}

func (h *Handler) withAuth(c *gin.Context) context.Context {
	ctx := context.WithValue(c.Request.Context(), stockclient.AuthHeaderKey, c.GetHeader("Authorization"))
	ctx = context.WithValue(ctx, salesclient.AuthHeaderKey, c.GetHeader("Authorization"))
	ctx = context.WithValue(ctx, purchasingclient.AuthHeaderKey, c.GetHeader("Authorization"))
	ctx = context.WithValue(ctx, configclient.AuthHeaderKey, c.GetHeader("Authorization"))
	ctx = context.WithValue(ctx, biclient.AuthHeaderKey, c.GetHeader("Authorization"))
	return context.WithValue(ctx, cashflowclient.AuthHeaderKey, c.GetHeader("Authorization"))
}

func intQuery(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func floatQuery(c *gin.Context, key string, def float64) float64 {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return n
}

func timeQuery(c *gin.Context, key string) *time.Time {
	v := c.Query(key)
	if v == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil
	}
	return &t
}

func (h *Handler) kits(c *gin.Context) {
	out, err := h.svc.KitsReport(h.withAuth(c))
	if err != nil {
		httpserver.Error(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) stock(c *gin.Context) {
	out, err := h.svc.StockReport(h.withAuth(c), c.Query("warehouse_id"))
	if err != nil {
		httpserver.Error(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) sales(c *gin.Context) {
	out, err := h.svc.SalesReport(h.withAuth(c), timeQuery(c, "from"), timeQuery(c, "to"))
	if err != nil {
		httpserver.Error(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) customerRanking(c *gin.Context) {
	out, err := h.svc.CustomerRankingReport(h.withAuth(c), timeQuery(c, "from"), timeQuery(c, "to"))
	if err != nil {
		httpserver.Error(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) customerDetail(c *gin.Context) {
	out, err := h.svc.CustomerDetailReport(h.withAuth(c), c.Param("id"), timeQuery(c, "from"), timeQuery(c, "to"))
	if err != nil {
		httpserver.Error(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) productSales(c *gin.Context) {
	out, err := h.svc.ProductSalesReport(h.withAuth(c), timeQuery(c, "from"), timeQuery(c, "to"))
	if err != nil {
		httpserver.Error(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) purchases(c *gin.Context) {
	out, err := h.svc.PurchasesReport(h.withAuth(c), timeQuery(c, "from"), timeQuery(c, "to"))
	if err != nil {
		httpserver.Error(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) losses(c *gin.Context) {
	out, err := h.svc.LossesReport(h.withAuth(c), timeQuery(c, "from"), timeQuery(c, "to"), c.Query("product_id"), c.Query("warehouse_id"))
	if err != nil {
		httpserver.Error(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) cashflow(c *gin.Context) {
	overdue := c.Query("overdue") == "true" || c.Query("overdue") == "1"
	out, err := h.svc.CashflowReport(h.withAuth(c), timeQuery(c, "from"), timeQuery(c, "to"), c.Query("direction"), c.Query("status"), overdue)
	if err != nil {
		httpserver.Error(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) cashflowTimeline(c *gin.Context) {
	out, err := h.svc.CashflowTimelineReport(h.withAuth(c), timeQuery(c, "from"), timeQuery(c, "to"))
	if err != nil {
		httpserver.Error(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) forecast(c *gin.Context) {
	coverage := intQuery(c, "coverage_weeks", 4)
	safety := floatQuery(c, "safety_percent", 0)
	lookback := intQuery(c, "lookback_weeks", 8)
	out, err := h.svc.ForecastReport(h.withAuth(c), coverage, safety, lookback)
	if err != nil {
		httpserver.Error(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
