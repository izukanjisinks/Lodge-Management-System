package models

// DashboardSummary backs GET /dashboard/summary — the 3 stat cards shown
// unconditionally on dashboard load, regardless of which tab is selected.
type DashboardSummary struct {
	NewBookingsThisMonth int `json:"new_bookings_this_month"`
	CheckInsToday        int `json:"checkins_today"`
	CheckOutsToday       int `json:"checkouts_today"`
}

type DashboardRevenuePoint struct {
	Month   string  `json:"month"` // "2026-01"
	Revenue float64 `json:"revenue"`
}

// DashboardReservationsBreakdown is the real booked/pending/cancelled split
// backing the Reservations donut chart (previously hardcoded mock data).
type DashboardReservationsBreakdown struct {
	Booked    int `json:"booked"`
	Pending   int `json:"pending"`
	Cancelled int `json:"cancelled"`
}

type DashboardRecentBooking struct {
	ID            string `json:"id"`
	BookingNumber string `json:"booking_number"`
	ClientName    string `json:"client_name"`
	BookerType    string `json:"booker_type"`
	RoomName      string `json:"room_name"`
	RoomType      string `json:"room_type"`
	VenueName     string `json:"venue_name,omitempty"`
	CheckIn       string `json:"check_in"`
	CheckOut      string `json:"check_out"`
	Status        string `json:"status"`
}

// DashboardBookings backs GET /dashboard/bookings — the Bookings tab.
type DashboardBookings struct {
	OverstayingGuests     int                            `json:"overstaying_guests"`
	PendingApprovals      int                            `json:"pending_approvals"`
	RevenueByMonth        []DashboardRevenuePoint        `json:"revenue_by_month"`
	ReservationsBreakdown DashboardReservationsBreakdown `json:"reservations_breakdown"`
	RecentBookings        []DashboardRecentBooking       `json:"recent_bookings"`
}

// DashboardOrderVolumePoint is one day's kitchen/bar order counts, backing
// the Order Volume bar chart (last 7 days).
type DashboardOrderVolumePoint struct {
	Day     string `json:"day"` // "Mon", "Tue", ...
	Kitchen int    `json:"kitchen"`
	Bar     int    `json:"bar"`
}

// DashboardOrdersByStation is the current open-order count per production
// station, backing the Orders by Station donut.
type DashboardOrdersByStation struct {
	Kitchen int `json:"kitchen"`
	Bar     int `json:"bar"`
	Bakery  int `json:"bakery"`
	Grill   int `json:"grill"`
}

type DashboardRecentOrder struct {
	ID          string `json:"id"`
	OrderNumber string `json:"order_number"`
	Guest       string `json:"guest"`
	Station     string `json:"station"` // kitchen | bar
	Items       int    `json:"items"`
	Status      string `json:"status"` // new | preparing | ready
	MinutesAgo  int    `json:"minutes_ago"`
}

// DashboardOrders backs GET /dashboard/orders — the Orders tab.
type DashboardOrders struct {
	KitchenBacklog int                         `json:"kitchen_backlog"`
	BarBacklog     int                         `json:"bar_backlog"`
	VolumeByDay    []DashboardOrderVolumePoint `json:"volume_by_day"`
	ByStation      DashboardOrdersByStation    `json:"by_station"`
	RecentOrders   []DashboardRecentOrder      `json:"recent_orders"`
}

// DashboardBilledVsCollectedPoint is one month's billed vs. actually-collected
// revenue, backing the Billed vs. Collected bar chart.
type DashboardBilledVsCollectedPoint struct {
	Month     string  `json:"month"` // "2026-01"
	Billed    float64 `json:"billed"`
	Collected float64 `json:"collected"`
}

// DashboardInvoicesByStatus is the total invoice amount currently in each
// status, backing the Invoices by Status donut.
type DashboardInvoicesByStatus struct {
	Paid      float64 `json:"paid"`
	Issued    float64 `json:"issued"`
	Overdue   float64 `json:"overdue"`
	Draft     float64 `json:"draft"`
	Cancelled float64 `json:"cancelled"`
}

type DashboardOutstandingInvoice struct {
	ID            string  `json:"id"`
	InvoiceNumber string  `json:"invoice_number"`
	Client        string  `json:"client"`
	Amount        float64 `json:"amount"`
	DueDate       string  `json:"due_date"`
	Status        string  `json:"status"` // issued | overdue
}

// DashboardInvoices backs GET /dashboard/invoices — the Invoices tab.
type DashboardInvoices struct {
	OverdueCount        int                               `json:"overdue_count"`
	OverdueAmount       float64                           `json:"overdue_amount"`
	DraftCount          int                               `json:"draft_count"`
	IssuedCount         int                               `json:"issued_count"`
	BilledVsCollected   []DashboardBilledVsCollectedPoint `json:"billed_vs_collected"`
	ByStatus            DashboardInvoicesByStatus         `json:"by_status"`
	OutstandingInvoices []DashboardOutstandingInvoice     `json:"outstanding_invoices"`
}
