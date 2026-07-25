package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"
	"lodge-system/internal/utils/email"
)

// BookingRequestApprover is satisfied by the individual and corporate booking
// request services. The workflow holds these as callbacks (keyed by TaskType) so a
// terminal workflow outcome can drive the same Approve/Reject path the request
// endpoints use — without WorkflowService importing those services (which would be
// a circular import, since they already depend on WorkflowService for InitiateWorkflow).
type BookingRequestApprover interface {
	ApproveFromWorkflow(ctx context.Context, id, orgID uuid.UUID) error
	RejectFromWorkflow(ctx context.Context, id, orgID uuid.UUID) error
}

type WorkflowService struct {
	workflowRepo *repository.WorkflowRepository
	instanceRepo *repository.WorkflowInstanceRepository
	taskRepo     *repository.AssignedTaskRepository
	historyRepo  *repository.WorkflowHistoryRepository
	userRepo     *repository.UserRepository
	clientRepo   *repository.ClientRepository
	guestRepo    *repository.GuestRepository
	emailService *email.EmailService

	// approvers maps a TaskType (e.g. "individual_booking", "corporate_booking") to
	// the service that materialises that request on a terminal workflow outcome.
	approvers map[string]BookingRequestApprover
}

func NewWorkflowService(
	workflowRepo *repository.WorkflowRepository,
	instanceRepo *repository.WorkflowInstanceRepository,
	taskRepo *repository.AssignedTaskRepository,
	historyRepo *repository.WorkflowHistoryRepository,
	userRepo *repository.UserRepository,
	clientRepo *repository.ClientRepository,
	guestRepo *repository.GuestRepository,
	emailService *email.EmailService,
) *WorkflowService {
	return &WorkflowService{
		workflowRepo: workflowRepo,
		instanceRepo: instanceRepo,
		taskRepo:     taskRepo,
		historyRepo:  historyRepo,
		userRepo:     userRepo,
		clientRepo:   clientRepo,
		guestRepo:    guestRepo,
		emailService: emailService,
		approvers:    make(map[string]BookingRequestApprover),
	}
}

// RegisterApprover wires a request service to a TaskType so that a terminal
// workflow outcome (final-step approve / reject) drives that service's
// Approve/Reject — materialising or rejecting the underlying booking request.
func (s *WorkflowService) RegisterApprover(taskType string, approver BookingRequestApprover) {
	s.approvers[taskType] = approver
}

// InitiateWorkflow starts a new workflow instance using workflow type.
// orgID scopes the workflow template lookup and stamps the instance.
func (s *WorkflowService) InitiateWorkflow(ctx context.Context,
	workflowType models.WorkflowType,
	taskDetails models.TaskDetails,
	initiatorID string,
	priority string,
	dueDate interface{},
	orgID string,
) (*models.WorkflowInstance, error) {
	workflow, err := s.workflowRepo.GetByType(workflowType, orgID)
	if err != nil {
		return nil, fmt.Errorf("workflow not found for type %s: %w", workflowType, err)
	}

	firstActionStep, firstTransition, initialStepID, err := s.workflowRepo.GetFirstActionStep(workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get first action step: %w", err)
	}

	instance := &models.WorkflowInstance{
		OrgID:         orgID,
		WorkflowID:    workflow.ID,
		CurrentStepID: firstActionStep.ID,
		Status:        "in_progress",
		TaskDetails:   taskDetails,
		CreatedBy:     initiatorID,
		Priority:      priority,
	}

	if err := s.instanceRepo.Create(instance); err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	assigneeID, err := s.determineAssignee(orgID, firstTransition.AllowedRoles, taskDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to determine assignee: %w", err)
	}

	task := &models.AssignedTask{
		OrgID:      orgID,
		InstanceID: instance.ID,
		StepID:     firstActionStep.ID,
		StepName:   firstActionStep.StepName,
		AssignedTo: assigneeID,
		AssignedBy: initiatorID,
		Status:     "pending",
	}
	if taskDetails.BranchID != "" {
		task.BranchID = &taskDetails.BranchID
	}

	if err := s.taskRepo.Create(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	assigneeUUID, _ := uuid.Parse(assigneeID)
	if assignee, err := s.userRepo.GetUserByID(assigneeUUID); err == nil {
		task.TaskDetails = &taskDetails
		go s.notifyTaskAssignment(task, assignee, instance)
	}

	initiatorName := taskDetails.SenderDetails.SenderName
	if initiatorName == "" {
		initiatorName = initiatorID
	}

	history := &models.WorkflowHistory{
		InstanceID:      instance.ID,
		FromStepID:      &initialStepID,
		ToStepID:        firstActionStep.ID,
		ActionTaken:     firstTransition.ActionName,
		PerformedBy:     initiatorID,
		PerformedByName: initiatorName,
		Comments:        fmt.Sprintf("Initiated %s workflow", workflow.Name),
	}

	if err := s.historyRepo.Create(history); err != nil {
		return nil, fmt.Errorf("failed to create history: %w", err)
	}

	return instance, nil
}

// ProcessAction processes an action on a workflow instance, scoped to org.
// action must be "approve" or "reject" — semantic intents, not DB action_name values.
// approve → advance along the first valid outbound transition from the current step.
// reject  → terminate the instance immediately regardless of step configuration.
func (s *WorkflowService) ProcessAction(ctx context.Context,
	instanceID string,
	action string,
	performedByID string,
	comments string,
	orgID string,
) error {
	if action != "approve" && action != "reject" {
		return errors.New("action must be 'approve' or 'reject'")
	}

	instance, err := s.instanceRepo.GetByID(instanceID, orgID)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}

	if instance.Status == "completed" || instance.Status == "cancelled" || instance.Status == "rejected" {
		return errors.New("workflow instance is already closed")
	}

	if instance.CurrentStepID == "" {
		return errors.New("workflow instance has no current step — data may be corrupt")
	}

	currentStep, err := s.workflowRepo.GetStepByID(instance.CurrentStepID)
	if err != nil {
		return fmt.Errorf("current step not found: %w", err)
	}
	if currentStep.ID == "" {
		return fmt.Errorf("step '%s' not found in workflow_steps", instance.CurrentStepID)
	}

	isRejection := action == "reject"
	isCompletingFinalStep := currentStep.Final && !isRejection

	var transition *models.WorkflowTransition
	var nextStep *models.WorkflowStep

	if isRejection || isCompletingFinalStep {
		// Instance terminates here — fetch any outbound transition only to check allowed_roles.
		nextStep = currentStep
		transitions, _ := s.workflowRepo.GetValidTransitions(currentStep.ID)
		if len(transitions) > 0 {
			transition = &transitions[0]
		}
	} else {
		// Approve on a non-final step — advance to the next step via the first valid transition.
		transitions, err := s.workflowRepo.GetValidTransitions(currentStep.ID)
		if err != nil || len(transitions) == 0 {
			return fmt.Errorf("no valid transition found from step '%s'", currentStep.StepName)
		}
		transition = &transitions[0]
		if transition.ToStepID == "" {
			return fmt.Errorf("transition '%s' has no destination step — workflow may be misconfigured", transition.ID)
		}
		nextStep, err = s.workflowRepo.GetStepByID(transition.ToStepID)
		if err != nil {
			return fmt.Errorf("next step not found: %w", err)
		}
		if nextStep.ID == "" {
			return fmt.Errorf("destination step '%s' not found in workflow_steps", transition.ToStepID)
		}
	}

	if nextStep.ID == "" {
		return errors.New("could not resolve next step — workflow may be misconfigured")
	}

	var allowedRoles []string
	if transition != nil {
		allowedRoles = transition.AllowedRoles
	}
	if err := s.checkPermission(performedByID, allowedRoles); err != nil {
		return err
	}

	newStatus := "in_progress"
	if isRejection {
		newStatus = "rejected"
	} else if isCompletingFinalStep {
		newStatus = "completed"
	}

	if err := s.instanceRepo.UpdateStep(instanceID, nextStep.ID, newStatus, instance.OrgID); err != nil {
		return fmt.Errorf("failed to update instance: %w", err)
	}

	activeTask, err := s.taskRepo.GetActiveTaskForInstance(instanceID, instance.OrgID)
	if err == nil && activeTask != nil {
		if err := s.taskRepo.Complete(activeTask.ID, instance.OrgID); err != nil {
			return fmt.Errorf("failed to complete task: %w", err)
		}
	}

	if isRejection || isCompletingFinalStep {
		if err := s.instanceRepo.Complete(instanceID, instance.OrgID); err != nil {
			return fmt.Errorf("failed to complete instance: %w", err)
		}

		// Drive the underlying booking request to its terminal state via the
		// registered approver for this TaskType. Approve materialises the booking;
		// reject marks the request rejected — the same paths the request endpoints use.
		if err := s.applyOutcomeToRequest(instance, isCompletingFinalStep); err != nil {
			// Non-fatal: the workflow itself has completed. Surface the failure but
			// don't roll back the task/instance transition.
			fmt.Printf("warning: failed to apply workflow outcome to %s %s: %v\n",
				instance.TaskDetails.TaskType, instance.TaskDetails.TaskID, err)
		}
	} else {
		assigneeID, err := s.determineAssignee(instance.OrgID, transition.AllowedRoles, instance.TaskDetails)
		if err != nil {
			return fmt.Errorf("failed to determine assignee: %w", err)
		}

		newTask := &models.AssignedTask{
			OrgID:      instance.OrgID,
			InstanceID: instanceID,
			StepID:     nextStep.ID,
			StepName:   nextStep.StepName,
			AssignedTo: assigneeID,
			AssignedBy: performedByID,
			Status:     "pending",
			DueDate:    instance.DueDate,
		}
		if instance.TaskDetails.BranchID != "" {
			newTask.BranchID = &instance.TaskDetails.BranchID
		}

		if err := s.taskRepo.Create(newTask); err != nil {
			return fmt.Errorf("failed to create new task: %w", err)
		}

		assigneeUUID, _ := uuid.Parse(assigneeID)
		if assignee, err := s.userRepo.GetUserByID(assigneeUUID); err == nil {
			newTask.TaskDetails = &instance.TaskDetails
			go s.notifyTaskAssignment(newTask, assignee, instance)
		}
	}

	performerUUID, err := uuid.Parse(performedByID)
	if err != nil {
		return fmt.Errorf("invalid performer ID: %w", err)
	}

	performer, err := s.userRepo.GetUserByID(performerUUID)
	if err != nil {
		return fmt.Errorf("failed to get performer: %w", err)
	}

	history := &models.WorkflowHistory{
		InstanceID:      instanceID,
		FromStepID:      &currentStep.ID,
		ToStepID:        nextStep.ID,
		ActionTaken:     action,
		PerformedBy:     performedByID,
		PerformedByName: performer.Email,
		Comments:        comments,
	}

	if err := s.historyRepo.Create(history); err != nil {
		return fmt.Errorf("failed to create history: %w", err)
	}

	return nil
}

// applyOutcomeToRequest drives the underlying booking request to its terminal state
// when a workflow instance closes. It delegates to the approver registered for the
// instance's TaskType: final-step approval calls Approve (which materialises the
// booking), any rejection calls Reject. A no-op when no approver is registered or the
// task details are incomplete.
func (s *WorkflowService) applyOutcomeToRequest(instance *models.WorkflowInstance, approved bool) error {
	td := instance.TaskDetails
	if td.TaskType == "" || td.TaskID == "" {
		return nil
	}

	approver, ok := s.approvers[td.TaskType]
	if !ok {
		// No approver registered for this TaskType — workflow is audit-only here.
		return nil
	}

	requestID, err := uuid.Parse(td.TaskID)
	if err != nil {
		return fmt.Errorf("invalid task id %q: %w", td.TaskID, err)
	}
	orgID, err := uuid.Parse(instance.OrgID)
	if err != nil {
		return fmt.Errorf("invalid org id %q: %w", instance.OrgID, err)
	}

	if approved {
		if err := approver.ApproveFromWorkflow(context.Background(), requestID, orgID); err != nil {
			return err
		}
		go s.notifyGuestBookingOutcome(instance.OrgID, td, models.BookingStatusConfirmed)
		return nil
	}
	if err := approver.RejectFromWorkflow(context.Background(), requestID, orgID); err != nil {
		return err
	}
	go s.notifyGuestBookingOutcome(instance.OrgID, td, models.BookingStatusCancelled)
	return nil
}

// GetAllOrgTasks retrieves all tasks in an org regardless of assignee, paginated.
func (s *WorkflowService) GetAllOrgTasks(ctx context.Context, orgID, branchID, statusFilter string, page, pageSize int) ([]models.AssignedTask, int, error) {
	return s.taskRepo.GetAllByOrg(orgID, branchID, statusFilter, pageSize, (page-1)*pageSize)
}

// GetMyTasks retrieves the tasks assigned to a user, scoped to org and paginated.
func (s *WorkflowService) GetMyTasks(ctx context.Context, orgID, userID, statusFilter string, page, pageSize int) ([]models.AssignedTask, int, error) {
	return s.taskRepo.GetByAssignee(orgID, userID, statusFilter, pageSize, (page-1)*pageSize)
}

// GetTaskByID retrieves a single task by ID, scoped to org (any assignee).
func (s *WorkflowService) GetTaskByID(ctx context.Context, taskID, orgID string) (*models.AssignedTask, error) {
	return s.taskRepo.GetByID(taskID, orgID)
}

// GetInstanceHistory retrieves the complete history of a workflow instance, scoped to org.
func (s *WorkflowService) GetInstanceHistory(ctx context.Context, instanceID, orgID string) ([]models.WorkflowHistory, error) {
	if _, err := s.instanceRepo.GetByID(instanceID, orgID); err != nil {
		return nil, fmt.Errorf("instance not found: %w", err)
	}
	return s.historyRepo.GetByInstanceID(instanceID)
}

// GetInstanceByTaskID retrieves a workflow instance by the associated task ID, scoped to org.
func (s *WorkflowService) GetInstanceByTaskID(ctx context.Context, taskID, orgID string) (*models.WorkflowInstance, error) {
	return s.instanceRepo.GetByTaskID(taskID, orgID)
}

// CancelInstance cancels the workflow instance associated with the given booking
// request ID. It is a best-effort operation — if no instance is found (e.g. the
// org has no workflow configured) the error is silently swallowed so the request
// cancellation still succeeds.
func (s *WorkflowService) CancelInstance(ctx context.Context, taskID, orgID string) error {
	instance, err := s.instanceRepo.GetByTaskID(taskID, orgID)
	if err != nil {
		return nil // no instance found — nothing to cancel
	}
	if instance.Status == "cancelled" || instance.Status == "completed" || instance.Status == "rejected" {
		return nil // already in a terminal state
	}
	return s.instanceRepo.Cancel(instance.ID, orgID)
}

// determineAssignee finds the user with the required role who has the fewest pending tasks,
// scoped to the org (and branch when available) so tasks are never assigned cross-org.
func (s *WorkflowService) determineAssignee(orgID string, allowedRoles []string, taskDetails models.TaskDetails) (string, error) {
	if len(allowedRoles) == 0 {
		return "", errors.New("no allowed roles defined for transition")
	}

	orgUUID, _ := uuid.Parse(orgID)

	for _, roleName := range allowedRoles {
		user, err := s.userRepo.GetUserWithFewestTasksByRole(orgUUID, roleName, taskDetails.BranchID)
		if err == nil && user != nil {
			return user.UserID.String(), nil
		}
	}

	return "", fmt.Errorf("no active user found with any of the allowed roles: %v", allowedRoles)
}

// checkPermission verifies the user's role is allowed to trigger the transition.
// If allowedRoles is empty, any authenticated user may act.
func (s *WorkflowService) checkPermission(userID string, allowedRoles []string) error {
	if len(allowedRoles) == 0 {
		return nil
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	user, err := s.userRepo.GetUserByID(userUUID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if user.Role == nil {
		return fmt.Errorf("user %s has no role assigned", user.Email)
	}

	for _, allowedRole := range allowedRoles {
		if user.Role.Name == allowedRole {
			return nil
		}
	}

	return fmt.Errorf("user %s does not have permission to perform this action", user.Email)
}

// notifyTaskAssignment sends an email when a task is assigned
// notifyGuestBookingOutcome emails the guest when their booking is approved or rejected.
func (s *WorkflowService) notifyGuestBookingOutcome(orgID string, details models.TaskDetails, entityStatus string) {
	if s.emailService == nil || s.clientRepo == nil {
		return
	}

	clientID, err := uuid.Parse(details.SenderDetails.SenderID)
	if err != nil {
		fmt.Printf("warning: invalid client ID in task details: %v\n", err)
		return
	}

	orgUUID, _ := uuid.Parse(orgID)
	profile, err := s.clientRepo.GetIndividualByID(clientID, orgUUID)
	if err != nil {
		fmt.Printf("warning: could not find guest profile for booking outcome email: %v\n", err)
		return
	}

	var subject, htmlBody string
	switch entityStatus {
	case models.BookingStatusConfirmed:
		subject = "Your Booking is Confirmed — Mwakwanda"
		htmlBody = email.BookingApprovedTemplate(profile.FullName, func() string {
			if details.TaskRef != "" {
				return details.TaskRef
			}
			return details.TaskID
		}(), details.TaskDescription)
	case models.BookingStatusCancelled:
		subject = "Booking Update — Mwakwanda"
		htmlBody = email.BookingRejectedTemplate(profile.FullName, func() string {
			if details.TaskRef != "" {
				return details.TaskRef
			}
			return details.TaskID
		}(), details.TaskDescription)
	default:
		return
	}

	if err := s.emailService.SendEmail([]string{profile.Email}, subject, htmlBody); err != nil {
		fmt.Printf("warning: failed to send booking outcome email to %s: %v\n", profile.Email, err)
	}
}

func (s *WorkflowService) notifyTaskAssignment(task *models.AssignedTask, assignee *models.User, instance *models.WorkflowInstance) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic in notifyTaskAssignment: %v\n", r)
		}
	}()

	if s.emailService == nil {
		return
	}

	var subject, htmlBody string

	if task.TaskDetails != nil && task.TaskDetails.TaskType == "booking" {
		subject = fmt.Sprintf("Booking Approval Required — %s", task.TaskDetails.SenderDetails.SenderName)
		bookingRef := task.TaskDetails.TaskRef
		if bookingRef == "" {
			bookingRef = task.TaskDetails.TaskID
		}
		htmlBody = email.BookingTaskAssignedTemplate(
			assignee.FullName,
			bookingRef,
			task.TaskDetails.TaskDescription,
			task.TaskDetails.SenderDetails.SenderName,
			task.TaskDetails.SenderDetails.Position,
		)
	} else {
		subject = fmt.Sprintf("New Task Assigned: %s", task.StepName)
		description := ""
		if task.TaskDetails != nil {
			description = task.TaskDetails.TaskDescription
		}
		htmlBody = email.GenericTaskAssignedTemplate(assignee.FullName, task.StepName, description)
	}

	if err := s.emailService.SendEmail([]string{assignee.Email}, subject, htmlBody); err != nil {
		fmt.Printf("Warning: Failed to send task assignment email: %v\n", err)
	}
}
