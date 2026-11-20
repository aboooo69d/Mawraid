package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/najm-x/nova/modules/audit/dto"
	"github.com/najm-x/nova/modules/audit/service"
	"github.com/najm-x/nova/pkg/respons"
)

type AuditHandler struct {
	auditService service.AuditService
}

func NewAuditHandler(auditService service.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

// GetLogs handles GET /api/v1/admin/audit-logs
func (h *AuditHandler) GetLogs(c *gin.Context) {
	var filter dto.AuditLogFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		respons.Fail(c, &respons.AppError{
			Status:  http.StatusBadRequest,
			Code:    2001,
			Message: "invalid filter parameters: " + err.Error(),
		})
		return
	}

	result, err := h.auditService.GetLogs(c.Request.Context(), filter)
	if err != nil {
		respons.Fail(c, &respons.AppError{
			Status:  http.StatusInternalServerError,
			Code:    2002,
			Message: "failed to retrieve audit logs: " + err.Error(),
		})
		return
	}

	respons.OK(c, result)
}

// GetLogByID handles GET /api/v1/admin/audit-logs/:id
func (h *AuditHandler) GetLogByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		respons.Fail(c, &respons.AppError{
			Status:  http.StatusBadRequest,
			Code:    2003,
			Message: "invalid log id format, must be UUID",
		})
		return
	}

	log, err := h.auditService.GetLogByID(c.Request.Context(), id)
	if err != nil {
		respons.Fail(c, &respons.AppError{
			Status:  http.StatusNotFound,
			Code:    2004,
			Message: "audit log not found",
		})
		return
	}

	respons.OK(c, log)
}
