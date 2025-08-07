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

// List handles GET /api/v1/executions
func (h *ExecutionHandler) List(c *gin.Context) {
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

	// Set defaults
	if params.Limit == 0 {
		params.Limit = 20
	}

	executions, total, err := h.executionService.ListExecutions(c.Request.Context(), &params)
	if err != nil {
		h.logger.Error("Failed to list executions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	// Convert to response format
	executionResponses := make([]models.ExecutionResponse, len(executions))
	for i, execution := range executions {
		executionResponses[i] = execution.ToResponse()
	}

	response := models.ExecutionListResponse{
		Executions: executionResponses,
		Total:      total,
		Limit:      params.Limit,
		Offset:     params.Offset,
	}

	c.JSON(http.StatusOK, response)
}

// Get handles GET /api/v1/executions/:id
func (h *ExecutionHandler) Get(c *gin.Context) {
	executionID := c.Param("id")
	if executionID == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"execution ID is required",
		).WithRequestID(getRequestID(c)))
		return
	}

	execution, err := h.executionService.GetExecution(c.Request.Context(), executionID)
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

// GetEvents handles GET /api/v1/executions/:id/events
func (h *ExecutionHandler) GetEvents(c *gin.Context) {
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
