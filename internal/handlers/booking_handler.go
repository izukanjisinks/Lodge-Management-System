package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"lodge-system/internal/middleware"
	"lodge-system/internal/models"
	"lodge-system/internal/services"
	"lodge-system/pkg/utils"

	"github.com/google/uuid"
)

type BookingHandler struct {
	service *services.BookingService
}

func NewBookingHandler(service *services.BookingService) *BookingHandler {
	return &BookingHandler{service: service}
}

// ─── Individual booking ───────────────────────────────────────────────────────

// CreateIndividual handles POST /api/v1/bookings/individual
func (h *BookingHandler) CreateIndividual(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID := middleware.GetBranchIDFromContext(r.Context())

	var req models.CreateIndividualBookingRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	booking, err := h.service.CreateIndividual(orgID, branchID, &req, nil)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, booking)
}

// CreateFromRequest handles POST /api/v1/booking-requests/{request_id}/materialise.
// Single-phase: the path value is the pending booking's ID; staff supply room
// assignments and this promotes the pending booking to confirmed in place.
func (h *BookingHandler) CreateFromRequest(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	branchID := middleware.GetBranchIDFromContext(r.Context())

	bookingID, err := uuid.Parse(r.PathValue("request_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}

	var matReq models.MaterialiseRequest
	if err := utils.DecodeJson(r, &matReq); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	booking, err := h.service.CreateFromBooking(orgID, branchID, bookingID, &matReq)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, booking)
}

// ─── List & get ───────────────────────────────────────────────────────────────

// List handles GET /api/v1/bookings.
// Supports filters booker_type, booking_type, status and an optional
// from/to (YYYY-MM-DD) created_at window. Pass format=csv to download all
// matching rows as a CSV with columns tailored to the booker/booking type.
func (h *BookingHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	p := utils.ParsePagination(r)
	bookerType := r.URL.Query().Get("booker_type")
	bookingType := r.URL.Query().Get("booking_type")
	status := r.URL.Query().Get("status")
	from, to := parseDateRange(r)

	if r.URL.Query().Get("format") == "csv" {
		h.exportCSV(w, orgID, bookerType, bookingType, status, from, to)
		return
	}

	bookings, total, err := h.service.List(orgID, bookerType, bookingType, status, from, to, p.Page, p.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     bookings,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    total,
	})
}

// ListForWebUser handles GET /api/v1/web/confirmed-bookings
func (h *BookingHandler) ListForWebUser(w http.ResponseWriter, r *http.Request) {
	webUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	p := utils.ParsePagination(r)

	bookings, total, err := h.service.ListForWebUser(webUserID, p.Page, p.PageSize)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, utils.PaginatedResponse{
		Data:     bookings,
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    total,
	})
}

// parseDateRange reads optional from/to (YYYY-MM-DD) query params. "to" is
// pushed to the end of its day so the range is inclusive.
func parseDateRange(r *http.Request) (from, to *time.Time) {
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			end := t.Add(24*time.Hour - time.Nanosecond)
			to = &end
		}
	}
	return from, to
}

// exportCSV streams all matching bookings as a CSV. Columns follow the
// back-office tables, which differ per booker/booking type: events have a
// venue column, individual stays have room + dates + nights, and corporate
// (non-event) bookings show the booking sub-type and who booked.
func (h *BookingHandler) exportCSV(w http.ResponseWriter, orgID uuid.UUID, bookerType, bookingType, status string, from, to *time.Time) {
	bookings, err := h.service.ListForExport(orgID, bookerType, bookingType, status, from, to)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Decide which shape to export. booking_type=event wins; otherwise the
	// booker_type (individual vs corporate) selects the layout.
	kind := "corporate"
	switch {
	case bookingType == models.BookingTypeEvent:
		kind = "event"
	case bookerType == models.BookerTypeIndividual:
		kind = "individual"
	}

	filename := fmt.Sprintf("bookings-%s-%s.csv", kind, time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	cw := csv.NewWriter(w)
	defer cw.Flush()

	money := func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
	date := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02")
	}
	leadAssignment := func(b models.Booking) *models.BookingRoomAssignment {
		if len(b.Assignments) > 0 {
			return &b.Assignments[0]
		}
		return nil
	}

	switch kind {
	case "individual":
		_ = cw.Write([]string{"Booking #", "Guest", "Email", "Room", "Check-In", "Check-Out", "Nights", "Total (ZMW)", "Status", "Branch"})
		for _, b := range bookings {
			room, checkIn, checkOut, nights := "", "", "", ""
			if a := leadAssignment(b); a != nil {
				room, checkIn, checkOut, nights = a.RoomName, date(a.CheckIn), date(a.CheckOut), strconv.Itoa(a.Nights)
			}
			_ = cw.Write([]string{b.BookingNumber, b.BookerName, b.BookerEmail, room, checkIn, checkOut, nights, money(b.TotalAmount), b.Status, b.BranchName})
		}
	case "event":
		_ = cw.Write([]string{"Booking #", "Company", "Venue", "Type", "Total (ZMW)", "Status", "Branch"})
		for _, b := range bookings {
			company := b.CompanyName
			if company == "" {
				company = b.BookerName
			}
			_ = cw.Write([]string{b.BookingNumber, company, b.VenueName, b.BookingType, money(b.TotalAmount), b.Status, b.BranchName})
		}
	default: // corporate
		_ = cw.Write([]string{"Booking #", "Company", "Booked By", "Type", "Total (ZMW)", "Status", "Branch"})
		for _, b := range bookings {
			bookedBy := b.BookerName
			if bookedBy == "" {
				bookedBy = b.ProfileName
			}
			_ = cw.Write([]string{b.BookingNumber, b.CompanyName, bookedBy, b.BookingType, money(b.TotalAmount), b.Status, b.BranchName})
		}
	}
}

// GetByID handles GET /api/v1/bookings/{id}
func (h *BookingHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}

	booking, err := h.service.GetByID(id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, booking)
}

// ─── Status ───────────────────────────────────────────────────────────────────

// UpdateStatus handles PUT /api/v1/bookings/{id}/status
func (h *BookingHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}

	var req models.UpdateBookingStatusRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.UpdateStatus(id, orgID, req.Status); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Status updated"})
}

// CheckIn handles PUT /api/v1/bookings/{id}/checkin
func (h *BookingHandler) CheckIn(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}

	if err := h.service.CheckIn(id, orgID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	booking, err := h.service.GetByID(id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, booking)
}

// CheckOut handles PUT /api/v1/bookings/{id}/checkout
func (h *BookingHandler) CheckOut(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}

	if err := h.service.CheckOut(id, orgID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	booking, err := h.service.GetByID(id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, booking)
}

// Cancel handles DELETE /api/v1/bookings/{id}
func (h *BookingHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}

	if err := h.service.Cancel(id, orgID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Booking cancelled"})
}

// ─── Room assignments ─────────────────────────────────────────────────────────

// AssignRoom handles POST /api/v1/bookings/{id}/assignments
func (h *BookingHandler) AssignRoom(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}

	var req models.CreateRoomAssignmentRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	assignment, err := h.service.AssignRoom(id, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, assignment)
}

// ListAssignments handles GET /api/v1/bookings/{id}/assignments
func (h *BookingHandler) ListAssignments(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}

	assignments, err := h.service.ListAssignments(id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, assignments)
}

// UpdateAssignment handles PUT /api/v1/bookings/{id}/assignments/{assign_id}
func (h *BookingHandler) UpdateAssignment(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}
	assignID, err := uuid.Parse(r.PathValue("assign_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid assignment ID")
		return
	}

	var req models.UpdateRoomAssignmentRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	assignment, err := h.service.UpdateAssignment(id, orgID, assignID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, assignment)
}

// RemoveAssignment handles DELETE /api/v1/bookings/{id}/assignments/{assign_id}
func (h *BookingHandler) RemoveAssignment(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}
	assignID, err := uuid.Parse(r.PathValue("assign_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid assignment ID")
		return
	}

	if err := h.service.RemoveAssignment(id, orgID, assignID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Assignment removed"})
}

// CheckInAssignment handles PUT /api/v1/bookings/{id}/assignments/{assign_id}/checkin
func (h *BookingHandler) CheckInAssignment(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}
	assignID, err := uuid.Parse(r.PathValue("assign_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid assignment ID")
		return
	}

	if err := h.service.CheckInAssignment(id, orgID, assignID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Room checked in"})
}

// CheckOutAssignment handles PUT /api/v1/bookings/{id}/assignments/{assign_id}/checkout
func (h *BookingHandler) CheckOutAssignment(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}
	assignID, err := uuid.Parse(r.PathValue("assign_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid assignment ID")
		return
	}

	if err := h.service.CheckOutAssignment(id, orgID, assignID); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Room checked out"})
}

// ─── Attendees ────────────────────────────────────────────────────────────────

// ListAttendees handles GET /api/v1/bookings/{id}/attendees
func (h *BookingHandler) ListAttendees(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}

	attendees, err := h.service.ListAttendees(id, orgID)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, attendees)
}

// AddAttendee handles POST /api/v1/bookings/{id}/attendees
func (h *BookingHandler) AddAttendee(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}

	var req models.CreateAttendeeRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	attendee, err := h.service.AddAttendee(id, orgID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, attendee)
}

// UpdateAttendee handles PUT /api/v1/bookings/{id}/attendees/{attendee_id}
func (h *BookingHandler) UpdateAttendee(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}
	attendeeID, err := uuid.Parse(r.PathValue("attendee_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid attendee ID")
		return
	}

	var req models.UpdateAttendeeRequest
	if err := utils.DecodeJson(r, &req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	attendee, err := h.service.UpdateAttendee(id, orgID, attendeeID, &req)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, attendee)
}

// RemoveAttendee handles DELETE /api/v1/bookings/{id}/attendees/{attendee_id}
func (h *BookingHandler) RemoveAttendee(w http.ResponseWriter, r *http.Request) {
	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid booking ID")
		return
	}
	attendeeID, err := uuid.Parse(r.PathValue("attendee_id"))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid attendee ID")
		return
	}

	if err := h.service.RemoveAttendee(id, orgID, attendeeID); err != nil {
		utils.RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Attendee removed"})
}
