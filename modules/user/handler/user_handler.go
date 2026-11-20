package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auditDto "github.com/najm-x/nova/modules/audit/dto"
	auditService "github.com/najm-x/nova/modules/audit/service"
	"github.com/najm-x/nova/modules/user/dto"
	"github.com/najm-x/nova/modules/user/service"
	"github.com/najm-x/nova/pkg/respons"
)

type UserHandler struct {
	userService  service.UserService
	auditService auditService.AuditService
}

func NewUserHandler(userService service.UserService, auditService auditService.AuditService) *UserHandler {
	return &UserHandler{
		userService:  userService,
		auditService: auditService,
	}
}

// GetAllUsers handles GET /api/v1/admin/users
func (h *UserHandler) GetAllUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	users, err := h.userService.GetAllUsers(c.Request.Context(), page, pageSize)
	if err != nil {
		respons.Fail(c, &respons.AppError{
			Status:  http.StatusInternalServerError,
			Code:    3001,
			Message: "failed to retrieve users: " + err.Error(),
		})
		return
	}

	respons.OK(c, users)
}

// GetUserById handles GET /api/v1/admin/users/:id
func (h *UserHandler) GetUserById(c *gin.Context) {
	id := c.Param("id")

	user, err := h.userService.GetUserById(c.Request.Context(), id)
	if err != nil {
		respons.Fail(c, &respons.AppError{
			Status:  http.StatusNotFound,
			Code:    3002,
			Message: "user not found: " + err.Error(),
		})
		return
	}

	respons.OK(c, user)
}

// UpdateUser handles PUT /api/v1/admin/users/:id
// Modifies user details and automatically dispatches an immutable audit log entry.
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respons.Fail(c, &respons.AppError{
			Status:  http.StatusBadRequest,
			Code:    3003,
			Message: "invalid request body: " + err.Error(),
		})
		return
	}

	// 1. Snapshot previous state before update
	oldUser, _ := h.userService.GetUserById(c.Request.Context(), id)

	// 2. Perform modification
	updatedUser, err := h.userService.UpdateUser(c.Request.Context(), id, req)
	if err != nil {
		respons.Fail(c, &respons.AppError{
			Status:  http.StatusBadRequest,
			Code:    3004,
			Message: "failed to update user: " + err.Error(),
		})
		return
	}

	// 3. Extract actor info from session
	var actorUUID *uuid.UUID
	if val, exists := c.Get("user_id"); exists && val != nil {
		if s, ok := val.(string); ok {
			if u, err := uuid.Parse(s); err == nil {
				actorUUID = &u
			}
		}
	}

	actorRole := "admin"
	if r, exists := c.Get("role"); exists && r != nil {
		if s, ok := r.(string); ok && s != "" {
			actorRole = s
		}
	}

	// 4. Log immutable audit trail
	reason := req.Reason
	if reason == "" {
		reason = "تعديل إداري لبيانات المستخدم"
	}

	if h.auditService != nil {
		_, _ = h.auditService.Log(c.Request.Context(), auditDto.CreateAuditLogInput{
			ActorID:    actorUUID,
			ActorRole:  actorRole,
			Action:     "UPDATE_USER_DATA",
			EntityType: auditDto.EntityUser,
			EntityID:   id,
			OldValues:  oldUser,
			NewValues:  updatedUser,
			Reason:     reason,
			IPAddress:  c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
		})
	}

	respons.OK(c, updatedUser)
}

// DeleteUser handles DELETE /api/v1/admin/users/:id
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	// Snapshot old user
	oldUser, _ := h.userService.GetUserById(c.Request.Context(), id)

	if err := h.userService.DeleteUser(c.Request.Context(), id); err != nil {
		respons.Fail(c, &respons.AppError{
			Status:  http.StatusBadRequest,
			Code:    3005,
			Message: "failed to delete user: " + err.Error(),
		})
		return
	}

	// Extract actor
	var actorUUID *uuid.UUID
	if val, exists := c.Get("user_id"); exists && val != nil {
		if s, ok := val.(string); ok {
			if u, err := uuid.Parse(s); err == nil {
				actorUUID = &u
			}
		}
	}

	actorRole := "admin"
	if r, exists := c.Get("role"); exists && r != nil {
		if s, ok := r.(string); ok && s != "" {
			actorRole = s
		}
	}

	if h.auditService != nil {
		_, _ = h.auditService.Log(c.Request.Context(), auditDto.CreateAuditLogInput{
			ActorID:    actorUUID,
			ActorRole:  actorRole,
			Action:     "DELETE_USER",
			EntityType: auditDto.EntityUser,
			EntityID:   id,
			OldValues:  oldUser,
			NewValues:  map[string]any{"deleted": true},
			Reason:     "حذف حساب مستخدم بواسطة الإدارة",
			IPAddress:  c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
		})
	}

	respons.OK(c, gin.H{"deleted": true, "id": id})
}
