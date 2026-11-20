package router

import (
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/modules/audit/handler"
)

// RegisterAuditRoutes registers protected administrative audit endpoints.
func RegisterAuditRoutes(rg *gin.RouterGroup, h *handler.AuditHandler) {
	adminAudit := rg.Group("/admin/audit-logs").Use(
		middleware.SessionHandler(),
		middleware.RequireAdmin(),
	)
	{
		adminAudit.GET("", h.GetLogs)
		adminAudit.GET("/:id", h.GetLogByID)
	}
}
