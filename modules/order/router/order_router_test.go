package orderRouter

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	orderHandler "github.com/najm-x/nova/modules/order/handler"
)

func TestRegisterOrderRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")

	RegisterOrderRoutes(api, orderHandler.NewOrderHandler(nil))

	expected := map[string]bool{
		http.MethodPost + " /api/v1/orders":                              false,
		http.MethodGet + " /api/v1/orders/me":                            false,
		http.MethodGet + " /api/v1/orders/me/:order_id":                  false,
		http.MethodPatch + " /api/v1/orders/me/:order_id/cancel":         false,
		http.MethodGet + " /api/v1/orders/provider":                      false,
		http.MethodGet + " /api/v1/orders/provider/:order_id":            false,
		http.MethodPatch + " /api/v1/orders/provider/:order_id/accept":   false,
		http.MethodPatch + " /api/v1/orders/provider/:order_id/reject":   false,
		http.MethodPatch + " /api/v1/orders/provider/:order_id/complete": false,
	}

	for _, route := range r.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := expected[key]; ok {
			expected[key] = true
		}
	}

	for key, found := range expected {
		if !found {
			t.Fatalf("expected route %s to be registered", key)
		}
	}
}
