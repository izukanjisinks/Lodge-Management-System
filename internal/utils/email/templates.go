package email

import "fmt"

const loginURL = "http://localhost:5173/login"

// shared layout constants
const (
	colorPrimary       = "#92400e" // brand brown — header gradient start
	colorPrimaryLight  = "#a16207" // amber — header gradient end, button
	colorAccent        = "#1a7c4e" // teal-green — success states
	colorAccentLight   = "#16a34a"
	colorDanger        = "#dc2626"
	colorDangerLight   = "#991b1b"
	colorBg            = "#fafaf9" // warm off-white page background
	colorCard          = "#fffbf9" // card background
	colorBorder        = "#f5f5f4" // subtle warm border
	colorInfoBox       = "#fef9f0" // info box background (warm amber tint)
	colorInfoBorder    = "#fde68a" // info box border
	colorSuccessBox    = "#f0fdf4"
	colorSuccessBorder = "#bbf7d0"
	colorWarningBox    = "#fffbeb"
	colorWarningBorder = "#fde68a"
	colorDangerBox     = "#fef2f2"
	colorDangerBorder  = "#fecaca"
	colorText          = "#1c1917" // warm near-black
	colorTextMuted     = "#78716c" // muted warm gray
	colorFooterBg      = "#f5f5f4"
	fontStack          = "Inter, Arial, Helvetica, sans-serif"
)

// headerGradient returns the CSS gradient string for email headers
func headerGradient(start, end string) string {
	return fmt.Sprintf("background:linear-gradient(135deg, %s, %s);", start, end)
}

// loginButton returns the reusable HTML login button block
func loginButton() string {
	return fmt.Sprintf(`
              <table width="100%%" cellpadding="0" cellspacing="0" style="margin:30px 0;">
                <tr>
                  <td align="center">
                    <a href="%s"
                       style="display:inline-block; padding:14px 28px; background-color:%s; color:#ffffff; text-decoration:none; font-size:15px; font-weight:600; border-radius:8px; font-family:%s;">
                       Login Now
                    </a>
                  </td>
                </tr>
              </table>`, loginURL, colorPrimaryLight, fontStack)
}

// emailWrapper wraps content in the shared outer shell (background, card, header, footer)
func emailWrapper(title, headerStyle, bodyContent string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>%s</title>
</head>
<body style="margin:0; padding:0; background-color:%s; font-family:%s;">

  <table width="100%%" cellpadding="0" cellspacing="0" style="background-color:%s; padding:40px 0;">
    <tr>
      <td align="center">

        <table width="600" cellpadding="0" cellspacing="0" style="background:%s; border-radius:10px; box-shadow:0 4px 16px rgba(0,0,0,0.08); overflow:hidden;">

          <tr>
            <td style="%s padding:30px; text-align:center;">
              <h1 style="color:#ffffff; margin:0; font-size:21px; font-weight:700; letter-spacing:0.3px; font-family:%s;">
                %s
              </h1>
            </td>
          </tr>

          <tr>
            <td style="padding:40px 32px; color:%s; font-size:15px; line-height:1.7; font-family:%s;">
              %s
            </td>
          </tr>

          <tr>
            <td style="background-color:%s; padding:18px 32px; text-align:center; font-size:12px; color:%s; font-family:%s; border-top:1px solid %s;">
              This is an automated message from Mwakwanda system. Please do not reply to this email.
            </td>
          </tr>

        </table>

      </td>
    </tr>
  </table>

</body>
</html>`,
		title,
		colorBg, fontStack,
		colorBg,
		colorCard,
		headerStyle, fontStack, title,
		colorText, fontStack,
		bodyContent,
		colorFooterBg, colorTextMuted, fontStack, colorBorder,
	)
}

// infoTable returns a styled detail table used inside email bodies
func infoTable(bgColor, borderColor string, rows ...string) string {
	rowsHTML := ""
	for _, r := range rows {
		rowsHTML += r
	}
	return fmt.Sprintf(`
              <table width="100%%" cellpadding="0" cellspacing="0" style="margin:25px 0;">
                <tr>
                  <td style="background-color:%s; border:1px solid %s; padding:20px; border-radius:8px;">
                    <table width="100%%" cellpadding="0" cellspacing="0">
                      %s
                    </table>
                  </td>
                </tr>
              </table>`, bgColor, borderColor, rowsHTML)
}

// infoRow returns a single label/value row for infoTable
func infoRow(label, value string) string {
	return fmt.Sprintf(`
                      <tr>
                        <td style="padding:7px 0; font-weight:600; color:%s; width:140px; font-size:14px;">%s</td>
                        <td style="padding:7px 0; color:%s; font-size:14px;">%s</td>
                      </tr>`, colorTextMuted, label, colorText, value)
}

// alertBox returns a coloured notice block
func alertBox(bgColor, borderColor, textColor, content string) string {
	return fmt.Sprintf(`
              <p style="color:%s; background:%s; border:1px solid %s; padding:13px 16px; border-radius:6px; font-size:14px; margin:20px 0;">
                %s
              </p>`, textColor, bgColor, borderColor, content)
}

// signature returns the closing sign-off block
func signature() string {
	return fmt.Sprintf(`
              <p style="margin-bottom:0; margin-top:28px;">
                Best regards,<br/>
                <strong style="color:%s;">Mwakwanda Lodge Management System</strong>
              </p>`, colorPrimary)
}

// ─── Public Templates ─────────────────────────────────────────────────────────

// WelcomeUserTemplate generates HTML for new user onboarding notification
func WelcomeUserTemplate(fullName, userEmail, temporaryPassword string) string {
	header := headerGradient(colorPrimary, colorPrimaryLight)
	body := fmt.Sprintf(`
              <p style="margin-top:0;">Hello %s,</p>
              <p>Your account has been created in the Mwakwanda Lodge Management System.</p>
              %s
              %s
              <p>If you have any questions, please contact your administrator.</p>
              <p>We look forward to working with you!</p>
              %s`,
		fullName,
		infoTable(colorInfoBox, colorInfoBorder,
			infoRow("Email:", userEmail),
			infoRow("Password:", temporaryPassword),
		),
		alertBox(colorWarningBox, colorWarningBorder, "#92400e",
			"<strong>Important:</strong> You will be required to change this password on first login."),
		signature(),
	)
	return emailWrapper("Welcome to Mwakwanda Lodge Management System", header, body)
}

// BackofficeWelcomeTemplate generates HTML for a new backoffice platform admin account
func BackofficeWelcomeTemplate(fullName, userEmail, temporaryPassword string) string {
	header := headerGradient(colorPrimary, colorPrimaryLight)
	body := fmt.Sprintf(`
              <p style="margin-top:0;">Hello %s,</p>
              <p>You have been granted access to the <strong>Mwakwanda Lodge Management Backoffice Portal</strong> — the platform administration console for managing lodge organisations and platform-level settings.</p>
              %s
              %s
              <p>If you believe this was sent in error, please contact the platform administrator immediately.</p>
              %s`,
		fullName,
		infoTable(colorInfoBox, colorInfoBorder,
			infoRow("Portal:", "Backoffice Admin Portal"),
			infoRow("Email:", userEmail),
			infoRow("Password:", temporaryPassword),
		),
		signature(),
	)
	return emailWrapper("Mwakwanda Lodge Management Backoffice Portal Access", header, body)
}

// PasswordResetTemplate generates HTML for password reset notification
func PasswordResetTemplate(temporaryPassword string) string {
	header := headerGradient(colorPrimary, colorPrimaryLight)
	body := fmt.Sprintf(`
              <p style="margin-top:0;">Hello,</p>
              <p>Your password has been <strong>reset by an administrator</strong>.</p>
              <table width="100%%" cellpadding="0" cellspacing="0" style="margin:25px 0;">
                <tr>
                  <td style="background-color:%s; border:1px dashed %s; padding:22px; text-align:center; border-radius:8px;">
                    <p style="margin:0; font-size:13px; color:%s; font-weight:600; text-transform:uppercase; letter-spacing:0.5px;">Temporary Password</p>
                    <p style="margin:10px 0 0; font-size:22px; font-weight:700; letter-spacing:3px; color:%s; font-family:monospace;">
                      %s
                    </p>
                  </td>
                </tr>
              </table>
              <p>You will be required to change this password when you next log in.</p>
              %s
              %s
              %s`,
		colorInfoBox, colorPrimaryLight,
		colorTextMuted,
		colorPrimary,
		temporaryPassword,
		loginButton(),
		alertBox(colorDangerBox, colorDangerBorder, colorDanger,
			"If you did not expect this change, please contact your administrator immediately."),
		signature(),
	)
	return emailWrapper("Password Reset Notification", header, body)
}

// GenericTaskAssignedTemplate generates HTML for generic workflow task assignment
func GenericTaskAssignedTemplate(recipientName, taskName, taskDescription string) string {
	header := headerGradient(colorPrimary, colorPrimaryLight)
	body := fmt.Sprintf(`
              <p style="margin-top:0;">Hello %s,</p>
              <p>A new task has been assigned to you for your action.</p>
              %s
              <p>Please log in to the Mwakwanda Lodge Management System to review and take action on this task.</p>
              %s
              %s`,
		recipientName,
		infoTable(colorInfoBox, colorInfoBorder,
			infoRow("Task:", taskName),
			infoRow("Description:", taskDescription),
		),
		loginButton(),
		signature(),
	)
	return emailWrapper("New Task Assigned", header, body)
}

// BookingTaskAssignedTemplate generates a rich email for staff when a booking approval task is assigned to them.
// description is the pre-formatted task description built by BookingService (e.g. "Review booking for John Mwale — 2026-05-01 to 2026-05-05 (2 guest(s))").
// senderName / senderType are the guest name and client type (individual/corporate).
func BookingTaskAssignedTemplate(recipientName, bookingID, description, senderName, senderType string) string {
	header := headerGradient(colorPrimary, colorPrimaryLight)
	body := fmt.Sprintf(`
              <p style="margin-top:0;">Hello %s,</p>
              <p>A new booking request has been submitted and requires your review.</p>
              %s
              %s
              <p>Please log in to the Mwakwanda Lodge Management System to approve or reject this booking.</p>
              %s
              %s`,
		recipientName,
		infoTable(colorInfoBox, colorInfoBorder,
			infoRow("Booking ID:", bookingID),
			infoRow("Guest:", senderName),
			infoRow("Client Type:", senderType),
			infoRow("Details:", description),
		),
		alertBox(colorWarningBox, colorWarningBorder, "#92400e",
			"<strong>Action required:</strong> This booking remains pending until you approve or reject it."),
		loginButton(),
		signature(),
	)
	return emailWrapper("Booking Approval Required", header, body)
}

// BookingApprovedTemplate notifies a guest that their booking has been confirmed.
func BookingApprovedTemplate(guestName, bookingID, details string) string {
	header := headerGradient(colorAccent, colorAccentLight)
	body := fmt.Sprintf(`
              <p style="margin-top:0;">Dear %s,</p>
              <p>Great news — your booking at <strong>Mwakwanda</strong> has been <strong>approved</strong> and is now confirmed.</p>
              %s
              %s
              <p>We look forward to welcoming you. If you have any questions, please don't hesitate to get in touch.</p>
              <p style="margin-bottom:0; margin-top:28px;">
                Warm regards,<br/>
                <strong style="color:%s;">Mwakwanda Lodge</strong>
              </p>`,
		guestName,
		infoTable(colorSuccessBox, colorSuccessBorder,
			infoRow("Booking ID:", bookingID),
			infoRow("Details:", details),
			infoRow("Status:", "Confirmed"),
		),
		alertBox(colorSuccessBox, colorSuccessBorder, colorAccent,
			"Your reservation is confirmed. Please check in at the front desk on your arrival date."),
		colorPrimary,
	)
	return emailWrapper("Booking Confirmed — Mwakwanda", header, body)
}

// BookingRejectedTemplate notifies a guest that their booking has not been approved.
func BookingRejectedTemplate(guestName, bookingID, details string) string {
	header := headerGradient(colorDangerLight, colorDanger)
	body := fmt.Sprintf(`
              <p style="margin-top:0;">Dear %s,</p>
              <p>We regret to inform you that your booking request at <strong>Mwakwanda</strong> could not be approved at this time.</p>
              %s
              <p>Please feel free to contact us directly or make a new reservation for alternative dates — we would love to host you.</p>
              <p style="margin-bottom:0; margin-top:28px;">
                Warm regards,<br/>
                <strong style="color:%s;">Mwakwanda Lodge</strong>
              </p>`,
		guestName,
		infoTable(colorWarningBox, colorWarningBorder,
			infoRow("Booking ID:", bookingID),
			infoRow("Details:", details),
			infoRow("Status:", "Not Approved"),
		),
		colorPrimary,
	)
	return emailWrapper("Booking Update — Mwakwanda", header, body)
}

// GuestPasswordResetTemplate sends a newly generated password to a guest who requested a reset.
func GuestPasswordResetTemplate(fullName, newPassword string) string {
	header := headerGradient(colorDangerLight, colorDanger)
	body := fmt.Sprintf(`
              <p style="margin-top:0;">Dear %s,</p>
              <p>We received a request to reset your password for your account at <strong>Mwakwanda</strong>.</p>
              <p>Your new temporary password is shown below. Please log in and update it to something memorable.</p>
              %s
              %s
              <p>If you did not request this reset, please contact us immediately.</p>
              <p style="margin-bottom:0; margin-top:28px;">
                Warm regards,<br/>
                <strong style="color:%s;">Mwakwanda Lodge</strong>
              </p>`,
		fullName,
		infoTable(colorWarningBox, colorWarningBorder,
			infoRow("New Password:", newPassword),
		),
		alertBox(colorWarningBox, colorWarningBorder, "#92400e",
			"<strong>Important:</strong> Log in and change this password as soon as possible."),
		colorPrimary,
	)
	return emailWrapper("Password Reset — Mwakwanda", header, body)
}

// GuestWelcomeTemplate generates the welcome email sent to guests who self-register on Mwakwanda website.
func GuestWelcomeTemplate(fullName string) string {
	header := headerGradient(colorPrimary, colorPrimaryLight)
	body := fmt.Sprintf(`
              <p style="margin-top:0;">Dear %s,</p>
              <p>Welcome to <strong>Mwakwanda</strong> — we're delighted to have you.</p>
              <p>Your account is ready. You can now browse our rooms, choose a meal plan, and make reservations at any time from our website.</p>
              %s
              <p style="margin-top:28px;">We look forward to hosting you.</p>
              <p style="margin-bottom:0; margin-top:28px;">
                Warm regards,<br/>
                <strong style="color:%s;">Mwakwanda Lodge</strong>
              </p>`,
		fullName,
		alertBox(colorSuccessBox, colorSuccessBorder, colorAccent,
			"Your profile is complete and your first reservation is just a few clicks away."),
		colorPrimary,
	)
	return emailWrapper("Welcome to Mwakwanda", header, body)
}

// PaymentConfirmationEmailTemplate notifies a client that their payment has been
// received and their invoice has been marked as paid.
func PaymentConfirmationEmailTemplate(orgName, clientName, invoiceNumber, paidDate, totalPaid string) string {
	header := headerGradient(colorAccent, colorAccentLight)

	if orgName == "" {
		orgName = "Lodge Management"
	}

	body := fmt.Sprintf(`
              <p style="margin-top:0;">Dear %s,</p>
              <p>Thank you — we have received your payment and your invoice with <strong>%s</strong> has been marked as <strong>paid</strong>.</p>
              %s
              %s
              <p>Please keep this email for your records. If you have any questions, feel free to reach out.</p>
              <p style="margin-bottom:0; margin-top:28px;">
                Warm regards,<br/>
                <strong style="color:%s;">%s</strong>
              </p>`,
		clientName,
		orgName,
		infoTable(colorSuccessBox, colorSuccessBorder,
			infoRow("Invoice No.:", invoiceNumber),
			infoRow("Amount Paid:", totalPaid),
			infoRow("Date Paid:", paidDate),
			infoRow("Status:", "Paid"),
		),
		alertBox(colorSuccessBox, colorSuccessBorder, colorAccent,
			"Your account is fully settled. No further action is required."),
		colorPrimary,
		orgName,
	)
	return emailWrapper("Payment Received — "+invoiceNumber, header, body)
}

// TestEmailTemplate generates the HTML body for a one-off SMTP configuration test,
// sent from the backoffice settings page to confirm outbound mail is working.
func TestEmailTemplate(recipientLabel string) string {
	header := headerGradient(colorPrimary, colorPrimaryLight)
	body := fmt.Sprintf(`
              <p style="margin-top:0;">Hello%s,</p>
              <p>Test email from Mwakwanda.</p>
              %s
              <p>If you received this message, your outgoing email configuration is working correctly.</p>
              %s`,
		recipientLabel,
		alertBox(colorSuccessBox, colorSuccessBorder, colorAccent,
			"This is a test message — no action is required."),
		signature(),
	)
	return emailWrapper("SMTP Test — Mwakwanda", header, body)
}

// InvoiceInfoRow is an exported helper so callers can build extra summary rows
// (e.g. accounting references) to pass into InvoiceEmailTemplate.
func InvoiceInfoRow(label, value string) string {
	return infoRow(label, value)
}

// InvoiceEmailTemplate notifies a client that their invoice is attached as a PDF.
// The full breakdown lives in the attached document; the body shows a summary.
// orgName is the lodge/organization that issued the invoice; it falls back to a
// generic label when empty.
func InvoiceEmailTemplate(orgName, clientName, invoiceNumber, issueDate, dueDate, totalDue string, accountingRows ...string) string {
	header := headerGradient(colorPrimary, colorPrimaryLight)

	if orgName == "" {
		orgName = "Lodge Management"
	}

	rows := []string{
		infoRow("Invoice No.:", invoiceNumber),
		infoRow("Issue Date:", issueDate),
		infoRow("Due Date:", dueDate),
		infoRow("Total Due:", totalDue),
	}
	rows = append(rows, accountingRows...)

	body := fmt.Sprintf(`
              <p style="margin-top:0;">Dear %s,</p>
              <p>Please find attached your invoice from <strong>%s</strong>. A summary is shown below; the complete breakdown is in the attached PDF.</p>
              %s
              %s
              <p>Kindly settle the amount due by the date indicated. If you have any questions about this invoice, please get in touch.</p>
              <p style="margin-bottom:0; margin-top:28px;">
                Warm regards,<br/>
                <strong style="color:%s;">%s</strong>
              </p>`,
		clientName,
		orgName,
		infoTable(colorInfoBox, colorInfoBorder, rows...),
		alertBox(colorWarningBox, colorWarningBorder, "#92400e",
			"<strong>Note:</strong> The attached PDF is your official invoice document for your records."),
		colorPrimary,
		orgName,
	)
	return emailWrapper("Invoice "+invoiceNumber+" — "+orgName, header, body)
}
