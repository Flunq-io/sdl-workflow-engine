package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/flunq-io/api/internal/models"
	"github.com/flunq-io/api/internal/services"
)

// ExecutionHandler handles execution-related HTTP requests
type ExecutionHandler struct {
	executionService *services.ExecutionService
	logger           *zap.Logger
}

// NewExecutionHandler creates a new execution handler
func NewExecutionHandler(executionService *services.ExecutionService, logger *zap.Logger) *ExecutionHandler {
	return &ExecutionHandler{
		executionService: executionService,
		logger:           logger,
	}
}

// List handles GET /api/v1/:tenant_id/executions
func (h *ExecutionHandler) List(c *gin.Context) {
	// Extract tenant_id from path parameter
	tenantID := c.Param("tenant_id")
	if tenantID == "" {
		h.logger.Error("Missing tenant_id in path")
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"Missing tenant_id in path",
		).WithRequestID(getRequestID(c)))
		return
	}

	var params models.ExecutionListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		h.logger.Error("Failed to bind execution list parameters", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			models.MessageValidationFailed,
		).WithDetails(map[string]interface{}{
			"validation_error": err.Error(),
		}).WithRequestID(getRequestID(c)))
		return
	}

	// Set tenant_id from path parameter
	params.TenantID = tenantID

	// Set defaults for pagination
	if params.Limit == 0 && params.Size == 0 {
		params.Limit = 20
	}
	if params.SortBy == "" {
		params.SortBy = "started_at"
	}
	if params.SortOrder == "" {
		params.SortOrder = "desc"
	}

	// Try the enhanced filtering first, fall back to basic listing if it fails
	executions, _, _, appliedFilters, err := h.executionService.ListExecutionsWithFilters(c.Request.Context(), &params)
	if err != nil {
		h.logger.Warn("Enhanced filtering failed, falling back to basic listing", zap.Error(err))

		// Fallback to basic listing
		basicExecutions, _, basicErr := h.executionService.ListExecutions(c.Request.Context(), &params)
		if basicErr != nil {
			h.logger.Error("Failed to list executions", zap.Error(basicErr))
			c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
				models.ErrorCodeInternalError,
				models.MessageInternalError,
			).WithRequestID(getRequestID(c)))
			return
		}

		// Use basic results with corrected total
		executions = basicExecutions
		appliedFilters = make(map[string]interface{})
	}

	// Convert to response format
	executionResponses := make([]models.ExecutionResponse, len(executions))
	for i, execution := range executions {
		executionResponses[i] = execution.ToResponse()
	}

	// Ensure total matches actual results for consistency
	actualTotal := len(executions)
	actualFilteredCount := len(executions)

	h.logger.Info("EXECUTION HANDLER DEBUG",
		zap.Int("executions_length", len(executions)),
		zap.Int("actual_total", actualTotal),
		zap.Int("actual_filtered_count", actualFilteredCount))

	// Create pagination metadata
	pagination := models.NewPaginationMeta(actualTotal, actualFilteredCount, params.PaginationParams)

	// Create filter metadata
	filters := models.NewFilterMeta(appliedFilters, actualFilteredCount)

	// Create sort metadata
	var sortMeta *models.SortMeta
	if params.SortBy != "" {
		sortMeta = &models.SortMeta{
			Field: params.SortBy,
			Order: params.SortOrder,
		}
	}

	response := models.ExecutionListResponse{
		Items:      executionResponses,
		Pagination: pagination,
		Filters:    filters,
		Sort:       sortMeta,
	}

	c.JSON(http.StatusOK, response)
}

// Get handles GET /api/v1/:tenant_id/executions/:id
func (h *ExecutionHandler) Get(c *gin.Context) {
	tenantID := c.Param("tenant_id")
	if tenantID == "" {
		h.logger.Error("Missing tenant_id in path")
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"Missing tenant_id in path",
		).WithRequestID(getRequestID(c)))
		return
	}

	executionID := c.Param("id")
	if executionID == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"execution ID is required",
		).WithRequestID(getRequestID(c)))
		return
	}

	execution, err := h.executionService.GetExecutionByTenant(c.Request.Context(), executionID, tenantID)
	if err != nil {
		h.logger.Error("Failed to get execution",
			zap.String("execution_id", executionID),
			zap.Error(err))

		if err == services.ErrExecutionNotFound {
			c.JSON(http.StatusNotFound, models.NewErrorResponse(
				models.ErrorCodeExecutionNotFound,
				models.MessageExecutionNotFound,
			).WithRequestID(getRequestID(c)))
			return
		}

		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	c.JSON(http.StatusOK, execution)
}

// Cancel handles POST /api/v1/executions/:id
func (h *ExecutionHandler) Cancel(c *gin.Context) {
	executionID := c.Param("id")
	if executionID == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"execution ID is required",
		).WithRequestID(getRequestID(c)))
		return
	}

	var req models.CancelExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Cancellation reason is optional, so we can proceed without it
		req.Reason = "User requested cancellation"
	}

	execution, err := h.executionService.CancelExecution(c.Request.Context(), executionID, req.Reason)
	if err != nil {
		h.logger.Error("Failed to cancel execution",
			zap.String("execution_id", executionID),
			zap.Error(err))

		if err == services.ErrExecutionNotFound {
			c.JSON(http.StatusNotFound, models.NewErrorResponse(
				models.ErrorCodeExecutionNotFound,
				models.MessageExecutionNotFound,
			).WithRequestID(getRequestID(c)))
			return
		}

		if err == services.ErrExecutionNotRunning {
			c.JSON(http.StatusBadRequest, models.NewErrorResponse(
				models.ErrorCodeInvalidStatus,
				"Execution is not running and cannot be cancelled",
			).WithRequestID(getRequestID(c)))
			return
		}

		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	h.logger.Info("Execution cancelled successfully",
		zap.String("execution_id", executionID),
		zap.String("reason", req.Reason))

	c.JSON(http.StatusOK, execution.ToResponse())
}

// GetEvents handles GET /api/v1/:tenant_id/executions/:id/events
func (h *ExecutionHandler) GetEvents(c *gin.Context) {
	tenantID := c.Param("tenant_id")
	if tenantID == "" {
		h.logger.Error("Missing tenant_id in path")
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"Missing tenant_id in path",
		).WithRequestID(getRequestID(c)))
		return
	}

	executionID := c.Param("id")
	if executionID == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"execution ID is required",
		).WithRequestID(getRequestID(c)))
		return
	}

	// Parse query parameters
	var params models.EventHistoryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		h.logger.Error("Failed to bind event history query parameters", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			models.MessageValidationFailed,
		).WithDetails(map[string]interface{}{
			"validation_error": err.Error(),
		}).WithRequestID(getRequestID(c)))
		return
	}

	// Set defaults
	if params.Limit == 0 {
		params.Limit = 100
	}

	// First verify tenant access to the execution
	_, err := h.executionService.GetExecutionByTenant(c.Request.Context(), executionID, tenantID)
	if err != nil {
		h.logger.Error("Failed to verify execution access",
			zap.String("execution_id", executionID),
			zap.String("tenant_id", tenantID),
			zap.Error(err))

		if err == services.ErrExecutionNotFound {
			c.JSON(http.StatusNotFound, models.NewErrorResponse(
				models.ErrorCodeExecutionNotFound,
				models.MessageExecutionNotFound,
			).WithRequestID(getRequestID(c)))
			return
		}

		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	events, err := h.executionService.GetExecutionEvents(c.Request.Context(), executionID, &params)
	if err != nil {
		h.logger.Error("Failed to get execution events",
			zap.String("execution_id", executionID),
			zap.Error(err))

		if err == services.ErrExecutionNotFound {
			c.JSON(http.StatusNotFound, models.NewErrorResponse(
				models.ErrorCodeExecutionNotFound,
				models.MessageExecutionNotFound,
			).WithRequestID(getRequestID(c)))
			return
		}

		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	response := models.EventHistoryResponse{
		Events: events,
		Count:  len(events),
		Since:  params.Since,
	}

	c.JSON(http.StatusOK, response)
}
