package handlers

import (
	"encoding/json"
	"lodge-system/internal/interfaces"
	"net/http"
	"time"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/pkg/utils"
)

type WorkflowHandler struct {
	service interfaces.WorkflowInterface
}

func NewWorkflowHandler(service interfaces.WorkflowInterface) *WorkflowHandler {
	return &WorkflowHandler{
		service: service,
	}
}

// GetMyTasks retrieves all tasks assigned to the authenticated user
func (h *WorkflowHandler) GetMyTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	// Get status filter from query params (optional)
	status := r.URL.Query().Get("status") // "pending", "completed", etc.
	p := utils.ParsePagination(r)

	tasks, total, err := h.service.GetMyTasks(r.Context(), orgID.String(), userID.String(), status, p.Page, p.PageSize)
	if err != nil {
		http.Error(w, "Failed to retrieve tasks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks":     tasks,
		"count":     len(tasks),
		"page":      p.Page,
		"page_size": p.PageSize,
		"total":     total,
	})
}

// GetAllOrgTasks retrieves all tasks in the org regardless of assignee.
// Each task includes assignee_name so the UI can label who it belongs to.
func (h *WorkflowHandler) GetAllOrgTasks(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	status := r.URL.Query().Get("status")
	branchID := r.URL.Query().Get("branch_id")
	p := utils.ParsePagination(r)

	tasks, total, err := h.service.GetAllOrgTasks(r.Context(), orgID.String(), branchID, status, p.Page, p.PageSize)
	if err != nil {
		http.Error(w, "Failed to retrieve tasks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks":     tasks,
		"count":     len(tasks),
		"page":      p.Page,
		"page_size": p.PageSize,
		"total":     total,
	})
}

// GetMyPendingTasks retrieves only pending tasks for the authenticated user
func (h *WorkflowHandler) GetMyPendingTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	p := utils.ParsePagination(r)
	tasks, total, err := h.service.GetMyTasks(r.Context(), orgID.String(), userID.String(), "pending", p.Page, p.PageSize)
	if err != nil {
		http.Error(w, "Failed to retrieve pending tasks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks":     tasks,
		"count":     len(tasks),
		"page":      p.Page,
		"page_size": p.PageSize,
		"total":     total,
	})
}

// ProcessActionRequest represents the request body for processing an action
type ProcessActionRequest struct {
	Action   string `json:"action"`   // "approve", "reject", "submit"
	Comments string `json:"comments"` // Optional comments
}

// ProcessAction handles workflow actions (approve, reject, etc.)
func (h *WorkflowHandler) ProcessAction(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get instance ID from path
	instanceID := r.PathValue("id")
	if instanceID == "" {
		http.Error(w, "Instance ID is required", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req ProcessActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate action
	if req.Action == "" {
		http.Error(w, "Action is required", http.StatusBadRequest)
		return
	}

	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	// Process the action
	err := h.service.ProcessAction(r.Context(), instanceID, req.Action, userID.String(), req.Comments, orgID.String())
	if err != nil {
		// Check for specific errors
		if err.Error() == "workflow instance is already closed" {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if err.Error() == "user does not have permission to perform this action" ||
			err.Error()[:4] == "user" && err.Error()[len(err.Error())-31:] == "does not have permission to perform this action" {
			http.Error(w, "Permission denied", http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Action processed successfully",
		"action":  req.Action,
	})
}

// GetInstanceHistory retrieves the complete history of a workflow instance
func (h *WorkflowHandler) GetInstanceHistory(w http.ResponseWriter, r *http.Request) {
	instanceID := r.PathValue("id")
	if instanceID == "" {
		http.Error(w, "Instance ID is required", http.StatusBadRequest)
		return
	}

	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	history, err := h.service.GetInstanceHistory(r.Context(), instanceID, orgID.String())
	if err != nil {
		http.Error(w, "Failed to retrieve history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"instance_id": instanceID,
		"history":     history,
		"count":       len(history),
	})
}

// InitiateWorkflowRequest represents the request to start a new workflow
type InitiateWorkflowRequest struct {
	WorkflowType models.WorkflowType `json:"workflow_type"` // e.g., "LEAVE_REQUEST", "EMPLOYEE_ONBOARDING"
	TaskDetails  models.TaskDetails  `json:"task_details"`
	Priority     string              `json:"priority"` // "low", "medium", "high", "urgent"
	DueDate      *time.Time          `json:"due_date"` // Optional
}

// InitiateWorkflow starts a new workflow instance (used internally by other services)
func (h *WorkflowHandler) InitiateWorkflow(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req InitiateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.WorkflowType == "" {
		http.Error(w, "Workflow type is required", http.StatusBadRequest)
		return
	}
	if req.TaskDetails.TaskID == "" || req.TaskDetails.TaskType == "" {
		http.Error(w, "Task details (task_id and task_type) are required", http.StatusBadRequest)
		return
	}

	// Default priority
	if req.Priority == "" {
		req.Priority = "medium"
	}

	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	// Initiate workflow
	instance, err := h.service.InitiateWorkflow(r.Context(), req.WorkflowType,
		req.TaskDetails,
		userID.String(),
		req.Priority,
		req.DueDate,
		orgID.String(),
	)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Workflow initiated successfully",
		"instance": instance,
	})
}

// GetInstanceByTaskID retrieves a workflow instance by the task ID
func (h *WorkflowHandler) GetInstanceByTaskID(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	instance, err := h.service.GetInstanceByTaskID(r.Context(), taskID, orgID.String())
	if err != nil {
		http.Error(w, "Workflow instance not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instance)
}

// GetTaskDetails retrieves detailed information about a specific task
func (h *WorkflowHandler) GetTaskDetails(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	// Resolve the task by ID within the org. Any staff member may view a task's
	// details (read-only); acting on it is enforced separately in ProcessAction.
	foundTask, err := h.service.GetTaskByID(r.Context(), taskID, orgID.String())
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Get the workflow instance for more context
	instance, err := h.service.GetInstanceByTaskID(r.Context(), foundTask.InstanceID, orgID.String())
	if err != nil {
		// Task found but instance not found - just return task
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(foundTask)
		return
	}

	// Get history for full context
	history, _ := h.service.GetInstanceHistory(r.Context(), instance.ID, orgID.String())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task":     foundTask,
		"instance": instance,
		"history":  history,
	})
}
