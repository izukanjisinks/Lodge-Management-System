package routes

import (
	"lodge-system/internal/handlers"
)

func RegisterRoutes(
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	roomHandler *handlers.RoomHandler,
	clientHandler *handlers.ClientHandler,
	bookingHandler *handlers.BookingHandler,
	invoiceHandler *handlers.InvoiceHandler,
	dashboardHandler *handlers.DashboardHandler,
	workflowHandler *handlers.WorkflowHandler,
	workflowAdminHandler *handlers.WorkflowAdminHandler,
	menuHandler *handlers.MenuHandler,
	orderHandler *handlers.OrderHandler,
	guestAuthHandler *handlers.GuestAuthHandler,
	reviewHandler *handlers.ReviewHandler,
	backofficeAuthHandler *handlers.BackofficeAuthHandler,
	backofficeUserHandler *handlers.BackofficeUserHandler,
	backofficeOrgHandler *handlers.BackofficeOrganizationHandler,
	auditLogHandler *handlers.AuditLogHandler,
	orgSettingsHandler *handlers.OrganizationSettingsHandler,
	emailTestHandler *handlers.EmailTestHandler,
	branchHandler *handlers.BranchHandler,
	orgHandler *handlers.OrganizationHandler,
	webUserHandler *handlers.WebUserAuthHandler,
	corProfileHandler *handlers.CorProfileHandler,
	corpBookingReqHandler *handlers.CorporateBookingRequestHandler,
	indvBookingReqHandler *handlers.IndividualBookingRequestHandler,
	venueHandler *handlers.VenueHandler,
) {
	RegisterPublicRoutes(authHandler)
	RegisterAuthRoutes(authHandler)
	RegisterUserRoutes(userHandler)
	RegisterRoomRoutes(roomHandler)
	RegisterClientRoutes(clientHandler)
	RegisterBookingRoutes(bookingHandler)
	RegisterInvoiceRoutes(invoiceHandler)
	RegisterDashboardRoutes(dashboardHandler)
	RegisterWorkflowRoutes(workflowHandler)
	RegisterWorkflowAdminRoutes(workflowAdminHandler)
	RegisterMenuRoutes(menuHandler)
	RegisterOrderRoutes(orderHandler)
	RegisterGuestRoutes(guestAuthHandler, roomHandler, menuHandler, venueHandler)
	RegisterReviewRoutes(reviewHandler)
	RegisterBackofficeRoutes(backofficeAuthHandler, backofficeUserHandler, backofficeOrgHandler)
	RegisterAuditLogRoutes(auditLogHandler)
	RegisterOrganizationSettingsRoutes(orgSettingsHandler, emailTestHandler)
	RegisterBranchRoutes(branchHandler)
	RegisterOrganizationRoutes(orgHandler)
	RegisterWebUserRoutes(webUserHandler)
	RegisterCorProfileRoutes(corProfileHandler, corpBookingReqHandler)
	RegisterIndividualBookingRequestRoutes(indvBookingReqHandler)
	RegisterVenueRoutes(venueHandler)
}
