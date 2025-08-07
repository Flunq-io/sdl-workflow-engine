package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/flunq-io/api/internal/models"
	"github.com/flunq-io/api/internal/services"
)

// WorkflowHandler handles workflow-related HTTP requests
type WorkflowHandler struct {
	workflowService *services.WorkflowService
	logger          *zap.Logger
}

// NewWorkflowHandler creates a new workflow handler
func NewWorkflowHandler(workflowService *services.WorkflowService, logger *zap.Logger) *WorkflowHandler {
	return &WorkflowHandler{
		workflowService: workflowService,
		logger:          logger,
	}
}

// Create handles POST /api/v1/workflows
func (h *WorkflowHandler) Create(c *gin.Context) {
	var req models.CreateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind create workflow request", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			models.MessageValidationFailed,
		).WithDetails(map[string]interface{}{
			"validation_error": err.Error(),
		}).WithRequestID(getRequestID(c)))
		return
	}

	workflow, err := h.workflowService.CreateWorkflow(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to create workflow", zap.Error(err))

		if validationErr, ok := err.(*models.ValidationError); ok {
			c.JSON(http.StatusBadRequest, models.NewErrorResponse(
				models.ErrorCodeValidation,
				validationErr.Message,
			).WithDetails(map[string]interface{}{
				"field": validationErr.Field,
			}).WithRequestID(getRequestID(c)))
			return
		}

		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	h.logger.Info("Workflow created successfully",
		zap.String("workflow_id", workflow.ID),
		zap.String("name", workflow.Name))

	c.JSON(http.StatusCreated, workflow)
}

// List handles GET /api/v1/workflows
func (h *WorkflowHandler) List(c *gin.Context) {
	var params models.WorkflowListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		h.logger.Error("Failed to bind workflow list parameters", zap.Error(err))
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

	workflows, total, err := h.workflowService.ListWorkflows(c.Request.Context(), &params)
	if err != nil {
		h.logger.Error("Failed to list workflows", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	response := models.WorkflowListResponse{
		Items:  workflows,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}

	c.JSON(http.StatusOK, response)
}

// Get handles GET /api/v1/workflows/:id
func (h *WorkflowHandler) Get(c *gin.Context) {
	workflowID := c.Param("id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"workflow ID is required",
		).WithRequestID(getRequestID(c)))
		return
	}

	workflow, err := h.workflowService.GetWorkflow(c.Request.Context(), workflowID)
	if err != nil {
		h.logger.Error("Failed to get workflow",
			zap.String("workflow_id", workflowID),
			zap.Error(err))

		if err == services.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, models.NewErrorResponse(
				models.ErrorCodeWorkflowNotFound,
				models.MessageWorkflowNotFound,
			).WithRequestID(getRequestID(c)))
			return
		}

		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	c.JSON(http.StatusOK, workflow)
}

// Update handles PUT /api/v1/workflows/:id
func (h *WorkflowHandler) Update(c *gin.Context) {
	workflowID := c.Param("id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"workflow ID is required",
		).WithRequestID(getRequestID(c)))
		return
	}

	var req models.UpdateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind update workflow request", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			models.MessageValidationFailed,
		).WithDetails(map[string]interface{}{
			"validation_error": err.Error(),
		}).WithRequestID(getRequestID(c)))
		return
	}

	workflow, err := h.workflowService.UpdateWorkflow(c.Request.Context(), workflowID, &req)
	if err != nil {
		h.logger.Error("Failed to update workflow",
			zap.String("workflow_id", workflowID),
			zap.Error(err))

		if err == services.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, models.NewErrorResponse(
				models.ErrorCodeWorkflowNotFound,
				models.MessageWorkflowNotFound,
			).WithRequestID(getRequestID(c)))
			return
		}

		if validationErr, ok := err.(*models.ValidationError); ok {
			c.JSON(http.StatusBadRequest, models.NewErrorResponse(
				models.ErrorCodeValidation,
				validationErr.Message,
			).WithDetails(map[string]interface{}{
				"field": validationErr.Field,
			}).WithRequestID(getRequestID(c)))
			return
		}

		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	h.logger.Info("Workflow updated successfully",
		zap.String("workflow_id", workflowID))

	c.JSON(http.StatusOK, workflow)
}

// Delete handles DELETE /api/v1/workflows/:id
func (h *WorkflowHandler) Delete(c *gin.Context) {
	workflowID := c.Param("id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"workflow ID is required",
		).WithRequestID(getRequestID(c)))
		return
	}

	err := h.workflowService.DeleteWorkflow(c.Request.Context(), workflowID)
	if err != nil {
		h.logger.Error("Failed to delete workflow",
			zap.String("workflow_id", workflowID),
			zap.Error(err))

		if err == services.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, models.NewErrorResponse(
				models.ErrorCodeWorkflowNotFound,
				models.MessageWorkflowNotFound,
			).WithRequestID(getRequestID(c)))
			return
		}

		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	h.logger.Info("Workflow deleted successfully",
		zap.String("workflow_id", workflowID))

	c.Status(http.StatusNoContent)
}

// Execute handles POST /api/v1/workflows/:id/execute
func (h *WorkflowHandler) Execute(c *gin.Context) {
	workflowID := c.Param("id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"workflow ID is required",
		).WithRequestID(getRequestID(c)))
		return
	}

	var req models.ExecuteWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind execute workflow request", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			models.MessageValidationFailed,
		).WithDetails(map[string]interface{}{
			"validation_error": err.Error(),
		}).WithRequestID(getRequestID(c)))
		return
	}

	execution, err := h.workflowService.ExecuteWorkflow(c.Request.Context(), workflowID, &req)
	if err != nil {
		h.logger.Error("Failed to execute workflow",
			zap.String("workflow_id", workflowID),
			zap.Error(err))

		if err == services.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, models.NewErrorResponse(
				models.ErrorCodeWorkflowNotFound,
				models.MessageWorkflowNotFound,
			).WithRequestID(getRequestID(c)))
			return
		}

		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	h.logger.Info("Workflow execution started",
		zap.String("workflow_id", workflowID),
		zap.String("execution_id", execution.ID))

	c.JSON(http.StatusAccepted, execution.ToResponse())
}

// GetEvents handles GET /api/v1/workflows/:id/events
func (h *WorkflowHandler) GetEvents(c *gin.Context) {
	workflowID := c.Param("id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"workflow ID is required",
		).WithRequestID(getRequestID(c)))
		return
	}

	var params models.EventHistoryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		h.logger.Error("Failed to bind event history parameters", zap.Error(err))
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

	events, err := h.workflowService.GetWorkflowEvents(c.Request.Context(), workflowID, &params)
	if err != nil {
		h.logger.Error("Failed to get workflow events",
			zap.String("workflow_id", workflowID),
			zap.Error(err))

		if err == services.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, models.NewErrorResponse(
				models.ErrorCodeWorkflowNotFound,
				models.MessageWorkflowNotFound,
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

// GetExecutions handles GET /api/v1/workflows/:id/executions
func (h *WorkflowHandler) GetExecutions(c *gin.Context) {
	workflowID := c.Param("id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			models.ErrorCodeValidation,
			"workflow ID is required",
		).WithRequestID(getRequestID(c)))
		return
	}

	// Get all executions for this workflow
	params := &models.ExecutionListParams{
		WorkflowID: workflowID,
	}

	executions, total, err := h.workflowService.GetWorkflowExecutions(c.Request.Context(), workflowID, params)
	if err != nil {
		h.logger.Error("Failed to get workflow executions",
			zap.String("workflow_id", workflowID),
			zap.Error(err))

		if err == services.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, models.NewErrorResponse(
				models.ErrorCodeWorkflowNotFound,
				models.MessageWorkflowNotFound,
			).WithRequestID(getRequestID(c)))
			return
		}

		c.JSON(http.StatusInternalServerError, models.NewErrorResponse(
			models.ErrorCodeInternalError,
			models.MessageInternalError,
		).WithRequestID(getRequestID(c)))
		return
	}

	// Convert to execution IDs for UI compatibility
	executionIDs := make([]string, len(executions))
	for i, execution := range executions {
		executionIDs[i] = execution.ID
	}

	response := map[string]interface{}{
		"execution_ids": executionIDs,
		"count":         total,
	}

	c.JSON(http.StatusOK, response)
}

// getRequestID extracts the request ID from the context
func getRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}
