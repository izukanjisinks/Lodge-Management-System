package handlers

import (
	"encoding/base64"
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/internal/services"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type InvoiceHandler struct {
	service *services.InvoiceService
}

func NewInvoiceHandler(service *services.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{service: service}
}

func (h *InvoiceHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	pag := utils.ParsePagination(r)
	status := r.URL.Query().Get("status")
	clientType := r.URL.Query().Get("client_type")

	branchID, err := middleware.ResolveBranchID(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	invoices, total, err := h.service.List(orgID, branchID, status, clientType, pag.Page, pag.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     invoices,
		Page:     pag.Page,
		PageSize: pag.PageSize,
		Total:    total,
	})
}

func (h *InvoiceHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid invoice ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	inv, err := h.service.GetByID(id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, inv)
}

func (h *InvoiceHandler) GetByBookingID(w http.ResponseWriter, r *http.Request) {
	bookingID, err := uuid.Parse(r.PathValue("booking_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	inv, err := h.service.GetByBookingID(bookingID, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, inv)
}

// SendEmailRequest carries the frontend-rendered PDF as a base64 string.
type SendEmailRequest struct {
	PDFBase64 string `json:"pdf_base64"`
}

func (h *InvoiceHandler) SendEmail(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid invoice ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req SendEmailRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.PDFBase64 == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing invoice PDF")
		return
	}

	// Accept either a raw base64 string or a data URL ("data:application/pdf;base64,...").
	payload := req.PDFBase64
	if i := indexOfBase64Comma(payload); i >= 0 {
		payload = payload[i+1:]
	}
	pdf, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid PDF encoding")
		return
	}

	if err := h.service.SendInvoiceEmail(id, orgID, pdf); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Invoice sent successfully"})
}

// indexOfBase64Comma returns the index of the comma separating a data-URL
// prefix from its base64 payload, or -1 if the string is not a data URL.
func indexOfBase64Comma(s string) int {
	if len(s) < 5 || s[:5] != "data:" {
		return -1
	}
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}

func (h *InvoiceHandler) SendPaymentConfirmation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid invoice ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	if err := h.service.SendPaymentConfirmationEmail(id, orgID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Payment confirmation sent"})
}

func (h *InvoiceHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid invoice ID")
		return
	}
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())

	var req models.UpdateInvoiceStatusRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.ParsePaidDate()

	inv, err := h.service.UpdateStatus(id, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, inv)
}

// PostBookingCharge handles POST /api/v1/bookings/{id}/charges — appends a general
// line item to the booking's invoice. Generic primitive used by resident meal
// collection and any future manual-charge UI.
func (h *InvoiceHandler) PostBookingCharge(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	bookingID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}
	var req models.PostBookingChargeRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	inv, err := h.service.PostBookingCharge(bookingID, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, inv)
}
