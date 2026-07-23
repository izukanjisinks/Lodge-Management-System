package repository

import (
	"database/sql"
	"fmt"
	"time"

	"lodge-system/internal/database"
	"lodge-system/internal/models"

	"github.com/google/uuid"
)

type DashboardRepository struct {
	db *sql.DB
}

func NewDashboardRepository() *DashboardRepository {
	return &DashboardRepository{db: database.DB}
}

// ─── Summary tab (always loaded) ───────────────────────────────────────────────

func (r *DashboardRepository) Summary(orgID uuid.UUID, branchID *uuid.UUID) (models.DashboardSummary, error) {
	var s models.DashboardSummary
	today := time.Now().Format("2006-01-02")

	query := `
		SELECT
		    (SELECT COUNT(*) FROM bookings b
		        WHERE b.org_id = $2 AND b.created_at >= DATE_TRUNC('month', NOW())%[1]s) AS new_bookings_this_month,
		    (SELECT COUNT(*) FROM booking_room_assignments bra
		        JOIN bookings b ON b.id = bra.booking_id
		        WHERE b.org_id = $2 AND bra.check_in::date = $1::date
		          AND bra.status IN ('confirmed','checked_in')%[1]s) AS checkins_today,
		    (SELECT COUNT(*) FROM booking_room_assignments bra
		        JOIN bookings b ON b.id = bra.booking_id
		        WHERE b.org_id = $2 AND bra.check_out::date = $1::date
		          AND bra.status = 'checked_in'%[1]s) AS checkouts_today`
	args := []interface{}{today, orgID}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND b.branch_id = $%d", len(args))
	}
	query = fmt.Sprintf(query, branchFilter)
	err := r.db.QueryRow(query, args...).Scan(&s.NewBookingsThisMonth, &s.CheckInsToday, &s.CheckOutsToday)
	return s, err
}

// ─── Bookings tab ───────────────────────────────────────────────────────────────

// OverstayingGuests counts bookings currently checked in whose stay has run
// past checkout. Combines the durable bookings.overstayed flag (set by the
// nightly job) with a live date check, same logic as the Room Status board.
func (r *DashboardRepository) OverstayingGuests(orgID uuid.UUID, branchID *uuid.UUID) (int, error) {
	args := []interface{}{orgID}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND b.branch_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT COUNT(DISTINCT b.id)
		FROM bookings b
		JOIN booking_room_assignments bra ON bra.booking_id = b.id
		WHERE b.org_id = $1 AND b.status = 'checked_in'
		  AND bra.status = 'checked_in'
		  AND (b.overstayed OR bra.check_out < CURRENT_DATE)%s`, branchFilter)
	var count int
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// PendingApprovals counts assigned tasks still awaiting action.
func (r *DashboardRepository) PendingApprovals(orgID uuid.UUID, branchID *uuid.UUID) (int, error) {
	args := []interface{}{orgID}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND branch_id = $%d", len(args))
	}
	query := fmt.Sprintf(`SELECT COUNT(*) FROM assigned_tasks WHERE org_id = $1 AND status = 'pending'%s`, branchFilter)
	var count int
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

func (r *DashboardRepository) RevenueByMonth(orgID uuid.UUID, branchID *uuid.UUID, months int) ([]models.DashboardRevenuePoint, error) {
	args := []interface{}{orgID, months}
	query := `
		SELECT
		    TO_CHAR(DATE_TRUNC('month', i.created_at), 'YYYY-MM') AS month,
		    COALESCE(SUM(i.total), 0) AS revenue
		FROM invoices i
		WHERE i.org_id = $1
		  AND i.status IN ('issued', 'paid')
		  AND i.created_at >= DATE_TRUNC('month', NOW()) - ($2 - 1) * INTERVAL '1 month'`
	if branchID != nil {
		args = append(args, *branchID)
		query += fmt.Sprintf(" AND i.branch_id = $%d", len(args))
	}
	query += ` GROUP BY DATE_TRUNC('month', i.created_at) ORDER BY DATE_TRUNC('month', i.created_at) ASC`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.DashboardRevenuePoint
	for rows.Next() {
		var p models.DashboardRevenuePoint
		if err := rows.Scan(&p.Month, &p.Revenue); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if points == nil {
		points = []models.DashboardRevenuePoint{}
	}
	return points, rows.Err()
}

// ReservationsBreakdown returns booked/pending/cancelled counts for bookings
// created in the last `days` days — backs the Reservations donut chart.
func (r *DashboardRepository) ReservationsBreakdown(orgID uuid.UUID, branchID *uuid.UUID, days int) (models.DashboardReservationsBreakdown, error) {
	var b models.DashboardReservationsBreakdown
	args := []interface{}{orgID, days}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND branch_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
		    COUNT(*) FILTER (WHERE status NOT IN ('cancelled', 'pending', 'rejected')) AS booked,
		    COUNT(*) FILTER (WHERE status = 'pending') AS pending,
		    COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled
		FROM bookings
		WHERE org_id = $1 AND created_at >= NOW() - ($2 || ' days')::interval%s`, branchFilter)
	err := r.db.QueryRow(query, args...).Scan(&b.Booked, &b.Pending, &b.Cancelled)
	return b, err
}

func (r *DashboardRepository) RecentBookings(orgID uuid.UUID, branchID *uuid.UUID, limit int) ([]models.DashboardRecentBooking, error) {
	args := []interface{}{orgID, limit}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND b.branch_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
		    b.id,
		    b.booking_number,
		    b.booker_name AS client_name,
		    b.booker_type,
		    COALESCE(r.name, '') AS room_name,
		    COALESCE(r.type::text, '')  AS room_type,
		    COALESCE(v.name, '') AS venue_name,
		    COALESCE(TO_CHAR(asg.check_in,  'Mon DD, YYYY'), TO_CHAR(ev.start_date, 'Mon DD, YYYY'), '') AS check_in,
		    COALESCE(TO_CHAR(asg.check_out, 'Mon DD, YYYY'), TO_CHAR(ev.end_date,   'Mon DD, YYYY'), '') AS check_out,
		    b.status
		FROM bookings b
		LEFT JOIN LATERAL (
		    SELECT bra.room_id, bra.check_in, bra.check_out
		    FROM booking_room_assignments bra
		    WHERE bra.booking_id = b.id
		    ORDER BY bra.check_in ASC
		    LIMIT 1
		) asg ON TRUE
		LEFT JOIN rooms r ON r.id = asg.room_id
		LEFT JOIN booking_events ev ON ev.booking_id = b.id
		LEFT JOIN venues v ON v.id = ev.venue_id
		WHERE b.org_id = $1 AND b.status <> 'pending'%s
		ORDER BY b.created_at DESC
		LIMIT $2`, branchFilter)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []models.DashboardRecentBooking
	for rows.Next() {
		var b models.DashboardRecentBooking
		var clientName sql.NullString
		if err := rows.Scan(&b.ID, &b.BookingNumber, &clientName, &b.BookerType, &b.RoomName, &b.RoomType,
			&b.VenueName, &b.CheckIn, &b.CheckOut, &b.Status); err != nil {
			return nil, err
		}
		if clientName.Valid {
			b.ClientName = clientName.String
		}
		bookings = append(bookings, b)
	}
	if bookings == nil {
		bookings = []models.DashboardRecentBooking{}
	}
	return bookings, rows.Err()
}

// ─── Orders tab ─────────────────────────────────────────────────────────────────

// kitchenBarBacklogSQL is shared between KitchenBacklog and BarBacklog — an open
// order counts toward a station's backlog if it has at least one item relevant
// to that station and that station hasn't finished preparing it yet. Mirrors
// the production_area / category fallback used by OrderRepository's list query.
const kitchenBacklogSQL = `
	SELECT COUNT(*) FROM orders o
	WHERE o.org_id = $1 AND o.status = 'open' AND o.kitchen_status != 'ready'
	  AND EXISTS (
	      SELECT 1 FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id
	      WHERE oi.order_id = o.id AND (
	          mi.production_area IN ('kitchen', 'bakery', 'grill')
	          OR (mi.production_area IS NULL AND COALESCE(mi.category, '') != 'drinks')
	      )
	  )%s`

const barBacklogSQL = `
	SELECT COUNT(*) FROM orders o
	WHERE o.org_id = $1 AND o.status = 'open' AND o.bar_status != 'ready'
	  AND EXISTS (
	      SELECT 1 FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id
	      WHERE oi.order_id = o.id AND (
	          mi.production_area = 'bar'
	          OR (mi.production_area IS NULL AND mi.category = 'drinks')
	      )
	  )%s`

func (r *DashboardRepository) KitchenBacklog(orgID uuid.UUID, branchID *uuid.UUID) (int, error) {
	args := []interface{}{orgID}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND o.branch_id = $%d", len(args))
	}
	var count int
	err := r.db.QueryRow(fmt.Sprintf(kitchenBacklogSQL, branchFilter), args...).Scan(&count)
	return count, err
}

func (r *DashboardRepository) BarBacklog(orgID uuid.UUID, branchID *uuid.UUID) (int, error) {
	args := []interface{}{orgID}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND o.branch_id = $%d", len(args))
	}
	var count int
	err := r.db.QueryRow(fmt.Sprintf(barBacklogSQL, branchFilter), args...).Scan(&count)
	return count, err
}

// OrderVolumeByDay returns per-day kitchen/bar order counts for the last `days`
// days — an order counts toward a station if it has at least one item relevant
// to that station (same production_area/category fallback as the backlog counts).
func (r *DashboardRepository) OrderVolumeByDay(orgID uuid.UUID, branchID *uuid.UUID, days int) ([]models.DashboardOrderVolumePoint, error) {
	args := []interface{}{orgID, days}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND o.branch_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
		    TO_CHAR(day::date, 'Dy') AS day,
		    COUNT(*) FILTER (WHERE o.id IS NOT NULL AND EXISTS (
		        SELECT 1 FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id
		        WHERE oi.order_id = o.id AND (
		            mi.production_area IN ('kitchen', 'bakery', 'grill')
		            OR (mi.production_area IS NULL AND COALESCE(mi.category, '') != 'drinks')
		        )
		    )) AS kitchen,
		    COUNT(*) FILTER (WHERE o.id IS NOT NULL AND EXISTS (
		        SELECT 1 FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id
		        WHERE oi.order_id = o.id AND (
		            mi.production_area = 'bar'
		            OR (mi.production_area IS NULL AND mi.category = 'drinks')
		        )
		    )) AS bar
		FROM generate_series(
		    (CURRENT_DATE - ($2 - 1) * INTERVAL '1 day')::date,
		    CURRENT_DATE,
		    '1 day'::interval
		) AS day
		LEFT JOIN orders o ON o.created_at::date = day::date AND o.org_id = $1%s
		GROUP BY day
		ORDER BY day ASC`, branchFilter)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.DashboardOrderVolumePoint
	for rows.Next() {
		var p models.DashboardOrderVolumePoint
		if err := rows.Scan(&p.Day, &p.Kitchen, &p.Bar); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if points == nil {
		points = []models.DashboardOrderVolumePoint{}
	}
	return points, rows.Err()
}

// OrdersByStation returns the count of currently-open orders relevant to each
// production station (an order can count toward more than one station).
func (r *DashboardRepository) OrdersByStation(orgID uuid.UUID, branchID *uuid.UUID) (models.DashboardOrdersByStation, error) {
	var s models.DashboardOrdersByStation
	args := []interface{}{orgID}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND o.branch_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
		    COUNT(*) FILTER (WHERE EXISTS (
		        SELECT 1 FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id
		        WHERE oi.order_id = o.id AND (mi.production_area = 'kitchen' OR (mi.production_area IS NULL AND COALESCE(mi.category, '') != 'drinks'))
		    )) AS kitchen,
		    COUNT(*) FILTER (WHERE EXISTS (
		        SELECT 1 FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id
		        WHERE oi.order_id = o.id AND (mi.production_area = 'bar' OR (mi.production_area IS NULL AND mi.category = 'drinks'))
		    )) AS bar,
		    COUNT(*) FILTER (WHERE EXISTS (
		        SELECT 1 FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id
		        WHERE oi.order_id = o.id AND mi.production_area = 'bakery'
		    )) AS bakery,
		    COUNT(*) FILTER (WHERE EXISTS (
		        SELECT 1 FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id
		        WHERE oi.order_id = o.id AND mi.production_area = 'grill'
		    )) AS grill
		FROM orders o
		WHERE o.org_id = $1 AND o.status = 'open'%s`, branchFilter)
	err := r.db.QueryRow(query, args...).Scan(&s.Kitchen, &s.Bar, &s.Bakery, &s.Grill)
	return s, err
}

// RecentOrders returns the most recently placed open orders, newest first.
func (r *DashboardRepository) RecentOrders(orgID uuid.UUID, branchID *uuid.UUID, limit int) ([]models.DashboardRecentOrder, error) {
	args := []interface{}{orgID, limit}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND o.branch_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
		    o.id, o.order_number,
		    COALESCE(NULLIF(att.full_name, ''), NULLIF(cd.company_name, ''), b.booker_name, 'Walk-in') AS guest,
		    CASE WHEN EXISTS (
		        SELECT 1 FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id
		        WHERE oi.order_id = o.id AND (mi.production_area = 'bar' OR (mi.production_area IS NULL AND mi.category = 'drinks'))
		    ) AND NOT EXISTS (
		        SELECT 1 FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id
		        WHERE oi.order_id = o.id AND (mi.production_area IN ('kitchen','bakery','grill') OR (mi.production_area IS NULL AND COALESCE(mi.category,'') != 'drinks'))
		    ) THEN 'bar' ELSE 'kitchen' END AS station,
		    COALESCE((SELECT SUM(oi.quantity) FROM order_items oi WHERE oi.order_id = o.id), 0) AS items,
		    CASE WHEN EXISTS (
		        SELECT 1 FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id
		        WHERE oi.order_id = o.id AND (mi.production_area = 'bar' OR (mi.production_area IS NULL AND mi.category = 'drinks'))
		    ) THEN o.bar_status ELSE o.kitchen_status END AS status,
		    GREATEST(0, FLOOR(EXTRACT(EPOCH FROM (NOW() - o.created_at)) / 60))::int AS minutes_ago
		FROM orders o
		LEFT JOIN bookings b ON b.id = o.booking_id
		LEFT JOIN cor_company_details cd ON cd.id = b.company_id
		LEFT JOIN booking_attendees att ON att.id = o.attendee_id
		WHERE o.org_id = $1 AND o.status = 'open'%s
		ORDER BY o.created_at DESC
		LIMIT $2`, branchFilter)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []models.DashboardRecentOrder
	for rows.Next() {
		var o models.DashboardRecentOrder
		if err := rows.Scan(&o.ID, &o.OrderNumber, &o.Guest, &o.Station, &o.Items, &o.Status, &o.MinutesAgo); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	if orders == nil {
		orders = []models.DashboardRecentOrder{}
	}
	return orders, rows.Err()
}

// ─── Invoices tab ───────────────────────────────────────────────────────────────

// InvoicesSummary mirrors the frontend's isInvoiceOverdue: an invoice counts as
// overdue once its due_date has passed, unless it's already paid/cancelled — an
// issued/draft invoice's own status doesn't reliably flip to 'overdue' itself.
func (r *DashboardRepository) InvoicesSummary(orgID uuid.UUID, branchID *uuid.UUID) (models.DashboardInvoices, error) {
	var s models.DashboardInvoices
	args := []interface{}{orgID}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND branch_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
		    COUNT(*) FILTER (WHERE status NOT IN ('paid','cancelled')
		        AND (status = 'overdue' OR (due_date IS NOT NULL AND due_date < NOW()))) AS overdue_count,
		    COALESCE(SUM(total) FILTER (WHERE status NOT IN ('paid','cancelled')
		        AND (status = 'overdue' OR (due_date IS NOT NULL AND due_date < NOW()))), 0) AS overdue_amount,
		    COUNT(*) FILTER (WHERE status = 'draft') AS draft_count,
		    COUNT(*) FILTER (WHERE status = 'issued') AS issued_count
		FROM invoices
		WHERE org_id = $1%s`, branchFilter)
	err := r.db.QueryRow(query, args...).Scan(&s.OverdueCount, &s.OverdueAmount, &s.DraftCount, &s.IssuedCount)
	return s, err
}

// BilledVsCollected returns, per month, the total billed (issued+paid invoices
// created that month) vs. actually collected (paid invoices only).
func (r *DashboardRepository) BilledVsCollected(orgID uuid.UUID, branchID *uuid.UUID, months int) ([]models.DashboardBilledVsCollectedPoint, error) {
	args := []interface{}{orgID, months}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND branch_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
		    TO_CHAR(DATE_TRUNC('month', created_at), 'YYYY-MM') AS month,
		    COALESCE(SUM(total) FILTER (WHERE status IN ('issued', 'paid', 'overdue')), 0) AS billed,
		    COALESCE(SUM(total) FILTER (WHERE status = 'paid'), 0) AS collected
		FROM invoices
		WHERE org_id = $1
		  AND created_at >= DATE_TRUNC('month', NOW()) - ($2 - 1) * INTERVAL '1 month'%s
		GROUP BY DATE_TRUNC('month', created_at)
		ORDER BY DATE_TRUNC('month', created_at) ASC`, branchFilter)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.DashboardBilledVsCollectedPoint
	for rows.Next() {
		var p models.DashboardBilledVsCollectedPoint
		if err := rows.Scan(&p.Month, &p.Billed, &p.Collected); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if points == nil {
		points = []models.DashboardBilledVsCollectedPoint{}
	}
	return points, rows.Err()
}

// InvoicesByStatus returns the total invoice amount currently in each status.
func (r *DashboardRepository) InvoicesByStatus(orgID uuid.UUID, branchID *uuid.UUID) (models.DashboardInvoicesByStatus, error) {
	var s models.DashboardInvoicesByStatus
	args := []interface{}{orgID}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND branch_id = $%d", len(args))
	}
	// "Overdue" isn't reliably its own status yet (see InvoicesSummary), so an
	// issued/draft invoice past its due_date is counted under Overdue instead
	// of its raw status here, matching the frontend's effectiveInvoiceStatus.
	query := fmt.Sprintf(`
		SELECT
		    COALESCE(SUM(total) FILTER (WHERE status = 'paid'), 0) AS paid,
		    COALESCE(SUM(total) FILTER (WHERE status = 'issued' AND NOT (due_date IS NOT NULL AND due_date < NOW())), 0) AS issued,
		    COALESCE(SUM(total) FILTER (WHERE status IN ('issued','draft','overdue')
		        AND (status = 'overdue' OR (due_date IS NOT NULL AND due_date < NOW()))), 0) AS overdue,
		    COALESCE(SUM(total) FILTER (WHERE status = 'draft' AND NOT (due_date IS NOT NULL AND due_date < NOW())), 0) AS draft,
		    COALESCE(SUM(total) FILTER (WHERE status = 'cancelled'), 0) AS cancelled
		FROM invoices
		WHERE org_id = $1%s`, branchFilter)
	err := r.db.QueryRow(query, args...).Scan(&s.Paid, &s.Issued, &s.Overdue, &s.Draft, &s.Cancelled)
	return s, err
}

// OutstandingInvoices returns unpaid invoices (issued or overdue), soonest due
// date first.
func (r *DashboardRepository) OutstandingInvoices(orgID uuid.UUID, branchID *uuid.UUID, limit int) ([]models.DashboardOutstandingInvoice, error) {
	args := []interface{}{orgID, limit}
	branchFilter := ""
	if branchID != nil {
		args = append(args, *branchID)
		branchFilter = fmt.Sprintf(" AND i.branch_id = $%d", len(args))
	}
	query := fmt.Sprintf(`
		SELECT
		    i.id, i.invoice_number,
		    COALESCE(NULLIF(cd.company_name, ''), b.booker_name, '') AS client,
		    i.total,
		    COALESCE(TO_CHAR(i.due_date, 'YYYY-MM-DD'), '') AS due_date,
		    CASE WHEN i.status = 'overdue' OR (i.due_date IS NOT NULL AND i.due_date < NOW())
		         THEN 'overdue' ELSE 'issued' END AS status
		FROM invoices i
		LEFT JOIN bookings b ON b.id = i.booking_id
		LEFT JOIN cor_company_details cd ON cd.id = b.company_id
		WHERE i.org_id = $1 AND i.status IN ('issued', 'draft', 'overdue')%s
		ORDER BY i.due_date ASC NULLS LAST
		LIMIT $2`, branchFilter)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []models.DashboardOutstandingInvoice
	for rows.Next() {
		var inv models.DashboardOutstandingInvoice
		if err := rows.Scan(&inv.ID, &inv.InvoiceNumber, &inv.Client, &inv.Amount, &inv.DueDate, &inv.Status); err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}
	if invoices == nil {
		invoices = []models.DashboardOutstandingInvoice{}
	}
	return invoices, rows.Err()
}
