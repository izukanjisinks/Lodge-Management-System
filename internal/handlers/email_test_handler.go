package handlers

import (
	"net/http"

	"lodge-system/internal/middleware"
	"lodge-system/internal/utils/email"
	"lodge-system/pkg/utils"
)

type EmailTestHandler struct {
	emailService *email.EmailService
}

func NewEmailTestHandler(emailService *email.EmailService) *EmailTestHandler {
	return &EmailTestHandler{emailService: emailService}
}

type sendTestEmailRequest struct {
	To string `json:"to,omitempty"`
}

// SendTest sends a one-off test email using the configured SMTP settings, so an
// admin can verify outbound mail works without triggering a real notification flow.
// If "to" is omitted in the body, it defaults to the requesting admin's own email.
func (h *EmailTestHandler) SendTest(w http.ResponseWriter, r *http.Request) {
	var req sendTestEmailRequest
	_ = utils.DecodeJson(r, &req) // body is optional; ignore decode errors on empty body

	to := req.To
	if to == "" {
		to, _ = middleware.GetEmailFromContext(r.Context())
	}
	if to == "" {
		utils.RespondError(w, http.StatusBadRequest, "\"to\" is required and no admin email was found on the session")
		return
	}

	htmlBody := email.TestEmailTemplate("")
	if err := h.emailService.SendEmail([]string{to}, "Test Email — Mwakwanda", htmlBody); err != nil {
		utils.RespondError(w, http.StatusBadGateway, "Failed to send test email: "+err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{
		"message": "Test email sent successfully",
		"to":      to,
	})
}
