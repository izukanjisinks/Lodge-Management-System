package main

import (
	"fmt"
	"log"
	"net/http"

	"lodge-system/internal/config"
	"lodge-system/internal/database"
	"lodge-system/internal/handlers"
	"lodge-system/internal/jobs"
	"lodge-system/internal/middleware"
	"lodge-system/internal/repositories"
	"lodge-system/internal/repository"
	"lodge-system/internal/routes"
	"lodge-system/internal/services"
	"lodge-system/internal/utils/email"
)

func main() {
	cfg := config.Load()

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)
	if err := database.Connect(connStr); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	log.Println("Database connected")

	// Repositories
	userRepo := repository.NewUserRepository()
	roleRepo := repository.NewRoleRepository()
	roomRepo := repository.NewRoomRepository()
	clientRepo := repository.NewClientRepository()
	bookingRepo := repository.NewBookingRepository()
	invoiceRepo := repository.NewInvoiceRepository()
	dashboardRepo := repository.NewDashboardRepository()
	auditLogRepo := repository.NewAuditLogRepository()
	auditLogHandler := handlers.NewAuditLogHandler(services.NewAuditLogService(auditLogRepo))
	orgSettingsRepo := repository.NewOrganizationSettingsRepository()
	orgSettingsHandler := handlers.NewOrganizationSettingsHandler(services.NewOrganizationSettingsService(orgSettingsRepo))

	workflowRepo := repository.NewWorkflowRepository()
	instanceRepo := repository.NewWorkflowInstanceRepository()
	taskRepo := repository.NewAssignedTaskRepository()
	historyRepo := repository.NewWorkflowHistoryRepository()

	passwordPolicyRepo := repositories.NewPasswordPolicyRepository()
	passwordHistoryRepo := repositories.NewPasswordHistoryRepository()

	// Services
	roleService := services.NewRoleService(userRepo, roleRepo)
	userService := services.NewUserService(userRepo, roleRepo)

	passwordPolicyService := services.NewPasswordPolicyService(passwordPolicyRepo, passwordHistoryRepo)
	log.Println("Password policy service initialized")

	userService.SetPasswordPolicyService(passwordPolicyService)

	emailService := email.NewEmailService(&cfg.Email)
	log.Println("Email service initialized")

	userService.SetEmailService(emailService)

	guestRepo := repository.NewGuestRepository()

	workflowService := services.NewWorkflowService(workflowRepo, instanceRepo, taskRepo, historyRepo, userRepo, clientRepo, guestRepo, emailService)

	// Seed predefined roles
	if err := roleService.InitializePredefinedRoles(); err != nil {
		log.Fatalf("Failed to initialize roles: %v", err)
	}
	log.Println("Roles initialized")

	// Handlers
	authHandler := handlers.NewAuthHandler(userService)
	userHandler := handlers.NewUserHandler(userService, roleService)
	roomHandler := handlers.NewRoomHandler(services.NewRoomService(roomRepo))
	bookingDocRepo := repository.NewBookingDocumentRepository()
	clientSvc := services.NewClientService(clientRepo)
	clientSvc.SetBookingRepository(bookingRepo)
	clientSvc.SetBookingDocumentRepository(bookingDocRepo)
	clientHandler := handlers.NewClientHandler(clientSvc)
	attendeeRepo := repository.NewBookingAttendeeRepository()
	assignmentRepo := repository.NewBookingRoomAssignmentRepository()
	corpBookingReqRepo := repository.NewCorporateBookingRequestRepository()
	corpGuestRepo := repository.NewCorporateGuestRepository()
	bookingEventRepo := repository.NewBookingEventRepository()
	venueRepo := repository.NewVenueRepository()
	orderRepo := repository.NewOrderRepository()
	orgRepo := repository.NewOrganizationRepository()

	invoiceSvc := services.NewInvoiceService(invoiceRepo, bookingRepo, roomRepo, assignmentRepo, bookingEventRepo, orderRepo)
	invoiceSvc.SetEmailService(emailService)         // email invoice PDFs to clients
	invoiceSvc.SetOrganizationRepository(orgRepo)    // brand invoice emails with the issuing lodge's name
	bookingSvc := services.NewBookingService(bookingRepo, attendeeRepo, assignmentRepo, corpBookingReqRepo, corpGuestRepo, bookingEventRepo, venueRepo)
	bookingSvc.SetInvoiceService(invoiceSvc) // auto-generate draft invoice on booking confirm/materialise
	bookingSvc.SetOrderRepository(orderRepo)  // approved meals requests materialise into orders
	bookingSvc.SetClientRepository(clientRepo) // approved bookings populate the individual client registry

	bookingHandler := handlers.NewBookingHandler(bookingSvc)
	invoiceHandler := handlers.NewInvoiceHandler(invoiceSvc)
	dashboardHandler := handlers.NewDashboardHandler(services.NewDashboardService(dashboardRepo))
	workflowHandler := handlers.NewWorkflowHandler(workflowService)
	workflowAdminHandler := handlers.NewWorkflowAdminHandler(workflowRepo)
	passwordPolicyHandler := handlers.NewPasswordPolicyHandler(passwordPolicyService, userService)

	guestAuthSvc := services.NewGuestAuthService(guestRepo)
	guestAuthSvc.SetEmailService(emailService)
	branchRepo := repository.NewBranchRepository()
	guestAuthHandler := handlers.NewGuestAuthHandler(guestAuthSvc, orgRepo, branchRepo)

	backofficeUserRepo := repository.NewBackofficeUserRepository()

	backofficeAuthSvc := services.NewBackofficeAuthService(backofficeUserRepo)
	backofficeAuthSvc.SetEmailService(emailService)

	backofficeUserSvc := services.NewBackofficeUserService(backofficeUserRepo)
	backofficeUserSvc.SetEmailService(emailService)

	backofficeOrgSvc := services.NewBackofficeOrganizationService(orgRepo, userRepo, roleRepo, branchRepo)
	backofficeOrgSvc.SetEmailService(emailService)

	backofficeAuthHandler := handlers.NewBackofficeAuthHandler(backofficeAuthSvc)
	backofficeUserHandler := handlers.NewBackofficeUserHandler(backofficeUserSvc)
	backofficeOrgHandler := handlers.NewBackofficeOrganizationHandler(backofficeOrgSvc)

	menuRepo := repository.NewMenuRepository()
	menuHandler := handlers.NewMenuHandler(services.NewMenuService(menuRepo))
	orderSvc := services.NewOrderService(orderRepo, invoiceRepo, bookingRepo, auditLogRepo)
	orderHandler := handlers.NewOrderHandler(orderSvc)

	// Resident meal collection (sessions, cards, collect)
	mealSessionRepo := repository.NewMealSessionRepository()
	mealCardRepo := repository.NewMealCardRepository()
	mealCollectionRepo := repository.NewMealCollectionRepository()
	mealCollectionSvc := services.NewMealCollectionService(mealSessionRepo, mealCardRepo, mealCollectionRepo, invoiceRepo, menuRepo, attendeeRepo)
	mealCollectionHandler := handlers.NewMealCollectionHandler(mealCollectionSvc)

	branchHandler := handlers.NewBranchHandler(services.NewBranchService(branchRepo))
	orgHandler := handlers.NewOrganizationHandler(backofficeOrgSvc)

	venueHandler := handlers.NewVenueHandler(services.NewVenueService(venueRepo))

	reviewRepo := repository.NewReviewRepository()
	reviewHandler := handlers.NewReviewHandler(services.NewReviewService(reviewRepo, bookingRepo))

	// Web user (website accounts)
	webUserRepo := repository.NewWebUserRepository()
	webUserAuthSvc := services.NewWebUserAuthService(webUserRepo, passwordPolicyService)
	webUserAuthSvc.SetEmailService(emailService)
	webUserAuthHandler := handlers.NewWebUserAuthHandler(webUserAuthSvc)

	// Corporate profile layer
	corCompanyRepo := repository.NewCorCompanyRepository()
	corBranchRepo := repository.NewCorBranchRepository()
	corProfileRepo := repository.NewCorProfileRepository()
	corpGuestRepo = repository.NewCorporateGuestRepository()
	corpBookingReqRepo = repository.NewCorporateBookingRequestRepository()
	corProfileSvc := services.NewCorProfileService(corCompanyRepo, corBranchRepo, corProfileRepo, corpGuestRepo)
	corpBookingReqSvc := services.NewCorporateBookingRequestService(corpBookingReqRepo, corpGuestRepo, corProfileSvc)
	corpBookingReqSvc.SetWorkflowService(workflowService)
	corpBookingReqSvc.SetVenueRepository(venueRepo)
	corpBookingReqSvc.SetMenuRepository(menuRepo) // resolve menu item names/prices for meals task display
	corpBookingReqSvc.SetBookingService(bookingSvc) // approve auto-creates event/conference bookings
	corProfileHandler := handlers.NewCorProfileHandler(corProfileSvc)
	corpBookingReqHandler := handlers.NewCorporateBookingRequestHandler(corpBookingReqSvc)

	// Individual booking requests
	indvBookingReqRepo := repository.NewIndividualBookingRequestRepository()
	indvBookingReqSvc := services.NewIndividualBookingRequestService(indvBookingReqRepo, roomRepo, bookingSvc)
	indvBookingReqSvc.SetWorkflowService(workflowService)
	indvBookingReqHandler := handlers.NewIndividualBookingRequestHandler(indvBookingReqSvc)
	walkInBookingHandler := handlers.NewWalkInBookingHandler(indvBookingReqSvc, corpBookingReqSvc)

	// Wire the booking-request services back into the workflow so a terminal workflow
	// outcome (final approve / reject) materialises or rejects the underlying request.
	// Keys must match the TaskType set in each service's startWorkflow.
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
