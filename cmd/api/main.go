package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"go.uber.org/zap"

	"lodge-system/internal/config"
	"lodge-system/internal/database"
	"lodge-system/internal/handlers"
	"lodge-system/internal/interfaces"
	"lodge-system/internal/jobs"
	applogger "lodge-system/internal/logger"
	"lodge-system/internal/middleware"
	loggermw "lodge-system/internal/middleware/logger"
	telemetrymw "lodge-system/internal/middleware/telemetry"
	"lodge-system/internal/repositories"
	"lodge-system/internal/repository"
	"lodge-system/internal/routes"
	"lodge-system/internal/services"
	"lodge-system/internal/utils/email"
)

func main() {
	cfg := config.Load()

	// Structured logging. Install as the global logger so both handler-injected
	// loggers and zap.L() calls in service decorators share one configured sink.
	appLog, err := applogger.New(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() { _ = appLog.Sync() }()
	zap.ReplaceGlobals(appLog)

	appLog.Info("Starting Lodge Management System",
		zap.String("env", cfg.Env),
		zap.String("port", cfg.ServerPort),
	)

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)
	if err := database.Connect(connStr); err != nil {
		appLog.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer database.Close()
	appLog.Info("Database connected", zap.String("host", cfg.DBHost), zap.String("db", cfg.DBName))

	// Repositories
	userRepo := repository.NewUserRepository()
	roleRepo := repository.NewRoleRepository()
	roomRepo := repository.NewRoomRepository()
	clientRepo := repository.NewClientRepository()
	bookingRepo := repository.NewBookingRepository()
	invoiceRepo := repository.NewInvoiceRepository()
	dashboardRepo := repository.NewDashboardRepository()
	auditLogRepo := repository.NewAuditLogRepository()
	auditLogSvc := services.NewAuditLogService(auditLogRepo)
	var auditLogIface interfaces.AuditLogInterface = auditLogSvc
	auditLogIface = loggermw.NewAuditLogLoggerMiddleware(auditLogIface)
	auditLogIface = telemetrymw.NewAuditLogTelemetryMiddleware(auditLogIface)
	auditLogHandler := handlers.NewAuditLogHandler(auditLogIface)

	orgSettingsRepo := repository.NewOrganizationSettingsRepository()
	orgSettingsSvc := services.NewOrganizationSettingsService(orgSettingsRepo)
	var orgSettingsIface interfaces.OrganizationSettingsInterface = orgSettingsSvc
	orgSettingsIface = loggermw.NewOrganizationSettingsLoggerMiddleware(orgSettingsIface)
	orgSettingsIface = telemetrymw.NewOrganizationSettingsTelemetryMiddleware(orgSettingsIface)
	orgSettingsHandler := handlers.NewOrganizationSettingsHandler(orgSettingsIface)

	workflowRepo := repository.NewWorkflowRepository()
	instanceRepo := repository.NewWorkflowInstanceRepository()
	taskRepo := repository.NewAssignedTaskRepository()
	historyRepo := repository.NewWorkflowHistoryRepository()

	passwordPolicyRepo := repositories.NewPasswordPolicyRepository()
	passwordHistoryRepo := repositories.NewPasswordHistoryRepository()

	// Services
	//
	// Every service below is decorated the same way before being handed to its
	// handler: concrete service -> logging middleware -> telemetry middleware.
	// Handlers depend on the generated interface (internal/interfaces), so they
	// can't tell they're talking to a decorator stack instead of the raw service.
	// The concrete *XService var is kept alongside its decorated interface
	// wherever other services wire against it directly (Set* DI methods, which
	// take the concrete type, not the interface).
	roleService := services.NewRoleService(userRepo, roleRepo)
	var roleIface interfaces.RoleInterface = roleService
	roleIface = loggermw.NewRoleLoggerMiddleware(roleIface)
	roleIface = telemetrymw.NewRoleTelemetryMiddleware(roleIface)

	userService := services.NewUserService(userRepo, roleRepo)

	passwordPolicyService := services.NewPasswordPolicyService(passwordPolicyRepo, passwordHistoryRepo)
	log.Println("Password policy service initialized")
	var passwordPolicyIface interfaces.PasswordPolicyInterface = passwordPolicyService
	passwordPolicyIface = loggermw.NewPasswordPolicyLoggerMiddleware(passwordPolicyIface)
	passwordPolicyIface = telemetrymw.NewPasswordPolicyTelemetryMiddleware(passwordPolicyIface)

	userService.SetPasswordPolicyService(passwordPolicyService)

	emailService := email.NewEmailService(&cfg.Email)
	log.Println("Email service initialized")

	userService.SetEmailService(emailService)
	emailTestHandler := handlers.NewEmailTestHandler(emailService)

	var userIface interfaces.UserInterface = userService
	userIface = loggermw.NewUserLoggerMiddleware(userIface)
	userIface = telemetrymw.NewUserTelemetryMiddleware(userIface)

	guestRepo := repository.NewGuestRepository()

	workflowService := services.NewWorkflowService(workflowRepo, instanceRepo, taskRepo, historyRepo, userRepo, clientRepo, guestRepo, emailService)
	var workflowIface interfaces.WorkflowInterface = workflowService
	workflowIface = loggermw.NewWorkflowLoggerMiddleware(workflowIface)
	workflowIface = telemetrymw.NewWorkflowTelemetryMiddleware(workflowIface)

	// Seed predefined roles
	if err := roleService.InitializePredefinedRoles(context.Background()); err != nil {
		log.Fatalf("Failed to initialize roles: %v", err)
	}
	log.Println("Roles initialized")

	// Handlers
	authHandler := handlers.NewAuthHandler(userIface)
	userHandler := handlers.NewUserHandler(userIface, roleIface)
	roomSvc := services.NewRoomService(roomRepo)
	var roomIface interfaces.RoomInterface = roomSvc
	roomIface = loggermw.NewRoomLoggerMiddleware(roomIface)
	roomIface = telemetrymw.NewRoomTelemetryMiddleware(roomIface)
	roomHandler := handlers.NewRoomHandler(roomIface)
	bookingDocRepo := repository.NewBookingDocumentRepository()
	clientSvc := services.NewClientService(clientRepo)
	clientSvc.SetBookingRepository(bookingRepo)
	clientSvc.SetBookingDocumentRepository(bookingDocRepo)
	var clientIface interfaces.ClientInterface = clientSvc
	clientIface = loggermw.NewClientLoggerMiddleware(clientIface)
	clientIface = telemetrymw.NewClientTelemetryMiddleware(clientIface)
	clientHandler := handlers.NewClientHandler(clientIface)
	attendeeRepo := repository.NewBookingAttendeeRepository()
	assignmentRepo := repository.NewBookingRoomAssignmentRepository()
	corpBookingReqRepo := repository.NewCorporateBookingRequestRepository()
	corpGuestRepo := repository.NewCorporateGuestRepository()
	bookingEventRepo := repository.NewBookingEventRepository()
	venueRepo := repository.NewVenueRepository()
	orderRepo := repository.NewOrderRepository()
	orgRepo := repository.NewOrganizationRepository()

	invoiceSvc := services.NewInvoiceService(invoiceRepo, bookingRepo, roomRepo, assignmentRepo, bookingEventRepo, orderRepo)
	invoiceSvc.SetEmailService(emailService)      // email invoice PDFs to clients
	invoiceSvc.SetOrganizationRepository(orgRepo) // brand invoice emails with the issuing lodge's name
	bookingSvc := services.NewBookingService(bookingRepo, attendeeRepo, assignmentRepo, corpBookingReqRepo, corpGuestRepo, bookingEventRepo, venueRepo)
	bookingSvc.SetInvoiceService(invoiceSvc)   // auto-generate draft invoice on booking confirm/materialise
	bookingSvc.SetOrderRepository(orderRepo)   // approved meals requests materialise into orders
	bookingSvc.SetClientRepository(clientRepo) // approved bookings populate the individual client registry
	invoiceSvc.SetBookingService(bookingSvc)   // cancelling an invoice cascades to cancel its booking

	var invoiceIface interfaces.InvoiceInterface = invoiceSvc
	invoiceIface = loggermw.NewInvoiceLoggerMiddleware(invoiceIface)
	invoiceIface = telemetrymw.NewInvoiceTelemetryMiddleware(invoiceIface)

	var bookingIface interfaces.BookingInterface = bookingSvc
	bookingIface = loggermw.NewBookingLoggerMiddleware(bookingIface)
	bookingIface = telemetrymw.NewBookingTelemetryMiddleware(bookingIface)

	bookingHandler := handlers.NewBookingHandler(bookingIface)
	invoiceHandler := handlers.NewInvoiceHandler(invoiceIface)
	dashboardSvc := services.NewDashboardService(dashboardRepo)
	var dashboardIface interfaces.DashboardInterface = dashboardSvc
	dashboardIface = loggermw.NewDashboardLoggerMiddleware(dashboardIface)
	dashboardIface = telemetrymw.NewDashboardTelemetryMiddleware(dashboardIface)
	dashboardHandler := handlers.NewDashboardHandler(dashboardIface)
	workflowHandler := handlers.NewWorkflowHandler(workflowIface)
	workflowAdminHandler := handlers.NewWorkflowAdminHandler(workflowRepo)
	passwordPolicyHandler := handlers.NewPasswordPolicyHandler(passwordPolicyIface, userIface)

	guestAuthSvc := services.NewGuestAuthService(guestRepo)
	guestAuthSvc.SetEmailService(emailService)
	var guestAuthIface interfaces.GuestAuthInterface = guestAuthSvc
	guestAuthIface = loggermw.NewGuestAuthLoggerMiddleware(guestAuthIface)
	guestAuthIface = telemetrymw.NewGuestAuthTelemetryMiddleware(guestAuthIface)
	branchRepo := repository.NewBranchRepository()
	guestAuthHandler := handlers.NewGuestAuthHandler(guestAuthIface, orgRepo, branchRepo)

	backofficeUserRepo := repository.NewBackofficeUserRepository()

	backofficeAuthSvc := services.NewBackofficeAuthService(backofficeUserRepo)
	backofficeAuthSvc.SetEmailService(emailService)
	var backofficeAuthIface interfaces.BackofficeAuthInterface = backofficeAuthSvc
	backofficeAuthIface = loggermw.NewBackofficeAuthLoggerMiddleware(backofficeAuthIface)
	backofficeAuthIface = telemetrymw.NewBackofficeAuthTelemetryMiddleware(backofficeAuthIface)

	backofficeUserSvc := services.NewBackofficeUserService(backofficeUserRepo)
	backofficeUserSvc.SetEmailService(emailService)
	var backofficeUserIface interfaces.BackofficeUserInterface = backofficeUserSvc
	backofficeUserIface = loggermw.NewBackofficeUserLoggerMiddleware(backofficeUserIface)
	backofficeUserIface = telemetrymw.NewBackofficeUserTelemetryMiddleware(backofficeUserIface)

	backofficeOrgSvc := services.NewBackofficeOrganizationService(orgRepo, userRepo, roleRepo, branchRepo)
	backofficeOrgSvc.SetEmailService(emailService)
	var backofficeOrgIface interfaces.BackofficeOrganizationInterface = backofficeOrgSvc
	backofficeOrgIface = loggermw.NewBackofficeOrganizationLoggerMiddleware(backofficeOrgIface)
	backofficeOrgIface = telemetrymw.NewBackofficeOrganizationTelemetryMiddleware(backofficeOrgIface)

	backofficeAuthHandler := handlers.NewBackofficeAuthHandler(backofficeAuthIface)
	backofficeUserHandler := handlers.NewBackofficeUserHandler(backofficeUserIface)
	backofficeOrgHandler := handlers.NewBackofficeOrganizationHandler(backofficeOrgIface)

	menuRepo := repository.NewMenuRepository()
	menuSvc := services.NewMenuService(menuRepo)
	var menuIface interfaces.MenuInterface = menuSvc
	menuIface = loggermw.NewMenuLoggerMiddleware(menuIface)
	menuIface = telemetrymw.NewMenuTelemetryMiddleware(menuIface)
	menuHandler := handlers.NewMenuHandler(menuIface)
	orderSvc := services.NewOrderService(orderRepo, invoiceRepo, bookingRepo, auditLogRepo)
	// Decorate the order service: service -> logging -> telemetry. The handler
	// depends on the interface, so it can't tell it's talking to a decorator stack.
	var orderIface interfaces.OrderInterface = orderSvc
	orderIface = loggermw.NewOrderLoggerMiddleware(orderIface)
	orderIface = telemetrymw.NewOrderTelemetryMiddleware(orderIface)
	orderHandler := handlers.NewOrderHandler(orderIface)

	// Resident meal collection (sessions, cards, collect)
	mealSessionRepo := repository.NewMealSessionRepository()
	mealCardRepo := repository.NewMealCardRepository()
	mealCollectionRepo := repository.NewMealCollectionRepository()
	mealCollectionSvc := services.NewMealCollectionService(mealSessionRepo, mealCardRepo, mealCollectionRepo, invoiceRepo, menuRepo, attendeeRepo)
	var mealCollectionIface interfaces.MealCollectionInterface = mealCollectionSvc
	mealCollectionIface = loggermw.NewMealCollectionLoggerMiddleware(mealCollectionIface)
	mealCollectionIface = telemetrymw.NewMealCollectionTelemetryMiddleware(mealCollectionIface)
	mealCollectionHandler := handlers.NewMealCollectionHandler(mealCollectionIface)

	branchSvc := services.NewBranchService(branchRepo)
	var branchIface interfaces.BranchInterface = branchSvc
	branchIface = loggermw.NewBranchLoggerMiddleware(branchIface)
	branchIface = telemetrymw.NewBranchTelemetryMiddleware(branchIface)
	// PrinterService has no logging/telemetry decorator (direct hardware I/O,
	// outside the service-interface refactor) — passed to the handler
	// alongside the decorated branch interface.
	branchHandler := handlers.NewBranchHandler(branchIface, services.NewPrinterService(branchRepo))
	orgHandler := handlers.NewOrganizationHandler(backofficeOrgIface)

	venueSvc := services.NewVenueService(venueRepo)
	var venueIface interfaces.VenueInterface = venueSvc
	venueIface = loggermw.NewVenueLoggerMiddleware(venueIface)
	venueIface = telemetrymw.NewVenueTelemetryMiddleware(venueIface)
	venueHandler := handlers.NewVenueHandler(venueIface)

	reviewRepo := repository.NewReviewRepository()
	reviewSvc := services.NewReviewService(reviewRepo, bookingRepo)
	var reviewIface interfaces.ReviewInterface = reviewSvc
	reviewIface = loggermw.NewReviewLoggerMiddleware(reviewIface)
	reviewIface = telemetrymw.NewReviewTelemetryMiddleware(reviewIface)
	reviewHandler := handlers.NewReviewHandler(reviewIface)

	// Web user (website accounts)
	webUserRepo := repository.NewWebUserRepository()
	webUserAuthSvc := services.NewWebUserAuthService(webUserRepo, passwordPolicyService)
	webUserAuthSvc.SetEmailService(emailService)
	var webUserAuthIface interfaces.WebUserAuthInterface = webUserAuthSvc
	webUserAuthIface = loggermw.NewWebUserAuthLoggerMiddleware(webUserAuthIface)
	webUserAuthIface = telemetrymw.NewWebUserAuthTelemetryMiddleware(webUserAuthIface)
	webUserAuthHandler := handlers.NewWebUserAuthHandler(webUserAuthIface)

	// Corporate profile layer
	corCompanyRepo := repository.NewCorCompanyRepository()
	corBranchRepo := repository.NewCorBranchRepository()
	corProfileRepo := repository.NewCorProfileRepository()
	corpGuestRepo = repository.NewCorporateGuestRepository()
	corpBookingReqRepo = repository.NewCorporateBookingRequestRepository()
	corProfileSvc := services.NewCorProfileService(corCompanyRepo, corBranchRepo, corProfileRepo, corpGuestRepo)
	var corProfileIface interfaces.CorProfileInterface = corProfileSvc
	corProfileIface = loggermw.NewCorProfileLoggerMiddleware(corProfileIface)
	corProfileIface = telemetrymw.NewCorProfileTelemetryMiddleware(corProfileIface)

	corpBookingReqSvc := services.NewCorporateBookingRequestService(corpBookingReqRepo, corpGuestRepo, corProfileSvc)
	corpBookingReqSvc.SetWorkflowService(workflowService)
	corpBookingReqSvc.SetVenueRepository(venueRepo)
	corpBookingReqSvc.SetMenuRepository(menuRepo)   // resolve menu item names/prices for meals task display
	corpBookingReqSvc.SetBookingService(bookingSvc) // approve auto-creates event/conference bookings
	var corpBookingReqIface interfaces.CorporateBookingRequestInterface = corpBookingReqSvc
	corpBookingReqIface = loggermw.NewCorporateBookingRequestLoggerMiddleware(corpBookingReqIface)
	corpBookingReqIface = telemetrymw.NewCorporateBookingRequestTelemetryMiddleware(corpBookingReqIface)

	corProfileHandler := handlers.NewCorProfileHandler(corProfileIface)
	corpBookingReqHandler := handlers.NewCorporateBookingRequestHandler(corpBookingReqIface)

	// Individual booking requests
	indvBookingReqRepo := repository.NewIndividualBookingRequestRepository()
	indvBookingReqSvc := services.NewIndividualBookingRequestService(indvBookingReqRepo, roomRepo, bookingSvc)
	indvBookingReqSvc.SetWorkflowService(workflowService)
	var indvBookingReqIface interfaces.IndividualBookingRequestInterface = indvBookingReqSvc
	indvBookingReqIface = loggermw.NewIndividualBookingRequestLoggerMiddleware(indvBookingReqIface)
	indvBookingReqIface = telemetrymw.NewIndividualBookingRequestTelemetryMiddleware(indvBookingReqIface)

	indvBookingReqHandler := handlers.NewIndividualBookingRequestHandler(indvBookingReqIface)
	walkInBookingHandler := handlers.NewWalkInBookingHandler(indvBookingReqIface, corpBookingReqIface)

	// Wire the booking-request services back into the workflow so a terminal workflow
	// outcome (final approve / reject) materialises or rejects the underlying request.
	// Keys must match the TaskType set in each service's startWorkflow. Registered
	// with the concrete services (BookingRequestApprover can't reference the
	// interfaces package without an import cycle), so these calls bypass the
	// logging/telemetry decorators — acceptable since it's an internal callback,
	// not a handler-facing path.
	workflowService.RegisterApprover("individual_booking", indvBookingReqSvc)
	workflowService.RegisterApprover("corporate_booking", corpBookingReqSvc)

	// Background jobs
	jobs.NewOverdueCheckoutJob(bookingRepo, invoiceRepo, auditLogRepo, orgSettingsRepo).Start()
	log.Println("Overdue checkout job scheduled")
	jobs.NewCloseOrdersJob(orderSvc, orgSettingsRepo).Start()
	log.Println("Close orders job scheduled")

	// Register routes
	routes.RegisterRoutes(authHandler,
		userHandler,
		roomHandler,
		clientHandler,
		bookingHandler,
		invoiceHandler,
		dashboardHandler,
		workflowHandler,
		workflowAdminHandler,
		menuHandler,
		orderHandler,
		guestAuthHandler,
		reviewHandler,
		backofficeAuthHandler,
		backofficeUserHandler,
		backofficeOrgHandler,
		auditLogHandler,
		orgSettingsHandler,
		emailTestHandler,
		branchHandler,
		orgHandler,
		webUserAuthHandler,
		corProfileHandler,
		corpBookingReqHandler,
		indvBookingReqHandler,
		venueHandler)
	routes.RegisterPasswordPolicyRoutes(passwordPolicyHandler)
	routes.RegisterWalkInBookingRoutes(walkInBookingHandler)
	routes.RegisterMealCollectionRoutes(mealCollectionHandler)

	// Apply CORS middleware globally
	handler := middleware.Logger(middleware.CORS(http.DefaultServeMux))

	addr := ":" + cfg.ServerPort
	log.Printf("Lodge Management System running on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
