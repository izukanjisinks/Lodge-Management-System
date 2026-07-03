package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"lodge-system/internal/database"
	"lodge-system/internal/models"

	"github.com/google/uuid"
)

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{db: database.DB}
}

// Create inserts a new order and all its items in a single transaction.
// Menu item prices are snapshotted at order time.
func (r *OrderRepository) Create(o *models.Order, items []models.PlaceOrderItemRequest, orgID uuid.UUID) (*models.Order, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	if err = r.insertInTx(tx, o, items, orgID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(o.ID, orgID)
}

// CreateInTx inserts an order and its items using an existing transaction.
// Use this when the attendees being referenced were also created in the same tx
// and haven't been committed yet.
func (r *OrderRepository) CreateInTx(tx *sql.Tx, o *models.Order, items []models.PlaceOrderItemRequest, orgID uuid.UUID) error {
	return r.insertInTx(tx, o, items, orgID)
}

func (r *OrderRepository) insertInTx(tx *sql.Tx, o *models.Order, items []models.PlaceOrderItemRequest, orgID uuid.UUID) error {
	o.ID = uuid.New()
	now := time.Now()
	o.CreatedAt = now
	o.UpdatedAt = now
	o.OrgID = orgID

	err := tx.QueryRow(`
		INSERT INTO orders (id, org_id, branch_id, booking_id, attendee_id, type, status, notes, scheduled_for, meal_period, serving_time, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING order_number`,
		o.ID, orgID, o.BranchID, o.BookingID, o.AttendeeID, o.Type, models.OrderStatusOpen, o.Notes, o.ScheduledFor, o.MealPeriod, sql.NullString{String: o.ServingTime, Valid: o.ServingTime != ""}, now, now,
	).Scan(&o.OrderNumber)
	if err != nil {
		return err
	}

	for _, req := range items {
		var price float64
		if err = tx.QueryRow(`SELECT price FROM menu_items WHERE id=$1 AND org_id=$2`, req.MenuItemID, orgID).Scan(&price); err != nil {
			return fmt.Errorf("menu item %s not found", req.MenuItemID)
		}
		subtotal := price * float64(req.Quantity)
		if _, err = tx.Exec(`
			INSERT INTO order_items (id, order_id, menu_item_id, attendee_id, quantity, unit_price, subtotal, notes, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			uuid.New(), o.ID, req.MenuItemID, req.AttendeeID, req.Quantity, price, subtotal, req.Notes, now,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *OrderRepository) GetByID(id uuid.UUID, orgID uuid.UUID) (*models.Order, error) {
	var o models.Order
	var branchID, bookingID, attendeeID uuid.NullUUID
	var notes, bookingNumber, roomName, clientName, companyName, attendeeName sql.NullString

	var scheduledFor models.NullDate
	var mealPeriod, servingTime sql.NullString
	err := r.db.QueryRow(`
		SELECT o.id, o.org_id, o.branch_id, o.booking_id, o.attendee_id, o.order_number, o.type, o.status, o.notes,
		       COALESCE((SELECT SUM(subtotal) FROM order_items WHERE order_id = o.id), 0) AS total,
		       b.booking_number,
		       asg.room_name,
		       COALESCE(NULLIF(att.full_name, ''), NULLIF(cd.company_name, ''), b.booker_name) AS client_name,
		       cd.company_name,
		       COALESCE(att.full_name, lead.full_name) AS attendee_name,
		       o.scheduled_for, o.meal_period, o.serving_time,
		       o.created_at, o.updated_at
		FROM orders o
		LEFT JOIN bookings            b    ON b.id = o.booking_id
		LEFT JOIN cor_company_details cd   ON cd.id = b.company_id
		LEFT JOIN booking_attendees   att  ON att.id = o.attendee_id
		LEFT JOIN LATERAL (
		    SELECT full_name FROM booking_attendees
		    WHERE booking_id = b.id AND is_lead_contact = true
		    LIMIT 1
		) lead ON TRUE
		LEFT JOIN LATERAL (
		    SELECT ro.name AS room_name
		    FROM booking_room_assignments bra
		    JOIN rooms ro ON ro.id = bra.room_id
		    WHERE bra.booking_id = b.id
		    ORDER BY bra.check_in ASC LIMIT 1
		) asg ON TRUE
		WHERE o.id=$1 AND o.org_id=$2`, id, orgID).
		Scan(&o.ID, &o.OrgID, &branchID, &bookingID, &attendeeID, &o.OrderNumber, &o.Type, &o.Status, &notes, &o.Total,
			&bookingNumber, &roomName, &clientName, &companyName, &attendeeName,
			&scheduledFor, &mealPeriod, &servingTime,
			&o.CreatedAt, &o.UpdatedAt)
	if scheduledFor.Valid {
		o.ScheduledFor = &scheduledFor.Time
	}
	if mealPeriod.Valid {
		o.MealPeriod = mealPeriod.String
	}
	if servingTime.Valid {
		o.ServingTime = servingTime.String
	}
	if err != nil {
		return nil, err
	}
	if branchID.Valid {
		o.BranchID = &branchID.UUID
	}
	if bookingID.Valid {
		o.BookingID = &bookingID.UUID
	}
	if attendeeID.Valid {
		o.AttendeeID = &attendeeID.UUID
	}
	if notes.Valid {
		o.Notes = notes.String
	}
	if bookingNumber.Valid {
		o.BookingNumber = bookingNumber.String
	}
	if roomName.Valid {
		o.RoomName = roomName.String
	}
	if clientName.Valid {
		o.ClientName = clientName.String
	}
	if companyName.Valid {
		o.CompanyName = companyName.String
	}
	if attendeeName.Valid {
		o.AttendeeName = attendeeName.String
	}
	o.Items, err = r.fetchItems(id)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *OrderRepository) List(orgID uuid.UUID, branchID *uuid.UUID, orderType, status string, bookingID *uuid.UUID, from, to *time.Time, page, pageSize int) ([]models.Order, int, error) {
	// Default to open orders when no status filter is provided
	if status == "" {
		status = models.OrderStatusOpen
	}

	args := []interface{}{orgID}
	extraFilters := []string{fmt.Sprintf("o.status = $%d", 2)}
	args = append(args, status)
	i := 3

	if branchID != nil {
		extraFilters = append(extraFilters, fmt.Sprintf("o.branch_id = $%d", i))
		args = append(args, *branchID)
		i++
	}
	if orderType != "" {
		extraFilters = append(extraFilters, fmt.Sprintf("o.type = $%d", i))
		args = append(args, orderType)
		i++
	}
	if bookingID != nil {
		extraFilters = append(extraFilters, fmt.Sprintf("o.booking_id = $%d", i))
		args = append(args, *bookingID)
		i++
	}
	if from != nil {
		extraFilters = append(extraFilters, fmt.Sprintf("o.created_at >= $%d", i))
		args = append(args, *from)
		i++
	}
	if to != nil {
		extraFilters = append(extraFilters, fmt.Sprintf("o.created_at <= $%d", i))
		args = append(args, *to)
		i++
	}

	extraWhere := ""
	if len(extraFilters) > 0 {
		extraWhere = " AND " + strings.Join(extraFilters, " AND ")
	}

	var total int
	if err := r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM orders o WHERE o.org_id = $1%s`, extraWhere), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT o.id, o.org_id, o.branch_id, o.booking_id, o.attendee_id, o.order_number, o.type, o.status, o.notes,
		       COALESCE((SELECT SUM(subtotal) FROM order_items WHERE order_id = o.id), 0) AS total,
		       b.booking_number,
		       asg.room_name,
		       COALESCE(NULLIF(att.full_name, ''), NULLIF(cd.company_name, ''), b.booker_name) AS client_name,
		       cd.company_name,
		       COALESCE(att.full_name, lead.full_name) AS attendee_name,
		       o.scheduled_for, o.meal_period, o.serving_time,
		       o.created_at, o.updated_at
		FROM orders o
		LEFT JOIN bookings            b    ON b.id = o.booking_id
		LEFT JOIN cor_company_details cd   ON cd.id = b.company_id
		LEFT JOIN booking_attendees   att  ON att.id = o.attendee_id
		LEFT JOIN LATERAL (
		    SELECT full_name FROM booking_attendees
		    WHERE booking_id = b.id AND is_lead_contact = true
		    LIMIT 1
		) lead ON TRUE
		LEFT JOIN LATERAL (
		    SELECT ro.name AS room_name
		    FROM booking_room_assignments bra
		    JOIN rooms ro ON ro.id = bra.room_id
		    WHERE bra.booking_id = b.id
		    ORDER BY bra.check_in ASC LIMIT 1
		) asg ON TRUE
		WHERE o.org_id = $1%s
		ORDER BY o.scheduled_for ASC NULLS LAST, o.created_at DESC
		LIMIT $%d OFFSET $%d`, extraWhere, i, i+1), append(args, pageSize, (page-1)*pageSize)...)

	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var o models.Order
		var brid, bid, aid uuid.NullUUID
		var notes, bookingNumber, roomName, clientName, companyName, attendeeName, mealPeriod, servingTime sql.NullString
		var scheduledFor models.NullDate
		if err := rows.Scan(&o.ID, &o.OrgID, &brid, &bid, &aid, &o.OrderNumber, &o.Type, &o.Status, &notes, &o.Total,
			&bookingNumber, &roomName, &clientName, &companyName, &attendeeName,
			&scheduledFor, &mealPeriod, &servingTime,
			&o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if brid.Valid {
			o.BranchID = &brid.UUID
		}
		if bid.Valid {
			o.BookingID = &bid.UUID
		}
		if aid.Valid {
			o.AttendeeID = &aid.UUID
		}
		if notes.Valid {
			o.Notes = notes.String
		}
		if bookingNumber.Valid {
			o.BookingNumber = bookingNumber.String
		}
		if roomName.Valid {
			o.RoomName = roomName.String
		}
		if clientName.Valid {
			o.ClientName = clientName.String
		}
		if companyName.Valid {
			o.CompanyName = companyName.String
		}
		if attendeeName.Valid {
			o.AttendeeName = attendeeName.String
		}
		if scheduledFor.Valid {
			o.ScheduledFor = &scheduledFor.Time
		}
		if mealPeriod.Valid {
			o.MealPeriod = mealPeriod.String
		}
		if servingTime.Valid {
			o.ServingTime = servingTime.String
		}
		orders = append(orders, o)
	}
	return orders, total, rows.Err()
}

// InHouseGuest is a flat projection of a checked-in room assignment with enough
// context for the order picker: one row per guest, individual or corporate.
type InHouseGuest struct {
	BookingID     uuid.UUID
	BookingNumber string
	AttendeeID    *uuid.UUID
	GuestName     string
	RoomName      string
	CompanyName   string
}

// ListCheckedInGuests returns one row per checked-in room assignment across all
// active bookings for the org. Individual bookings produce one row (booker_name);
// corporate bookings produce one row per delegate (attendee full_name).
func (r *OrderRepository) ListCheckedInGuests(orgID uuid.UUID) ([]InHouseGuest, error) {
	rows, err := r.db.Query(`
		SELECT
		    b.id                                                           AS booking_id,
		    b.booking_number,
		    att.id                                                         AS attendee_id,
		    COALESCE(att.full_name, b.booker_name)                        AS guest_name,
		    COALESCE(ro.name, '')                                          AS room_name,
		    COALESCE(NULLIF(cd.company_name, ''), '')                     AS company_name
		FROM booking_room_assignments bra
		JOIN bookings            b   ON b.id  = bra.booking_id
		JOIN rooms               ro  ON ro.id = bra.room_id
		LEFT JOIN booking_attendees  att ON att.id = bra.attendee_id
		LEFT JOIN cor_company_details cd ON cd.id  = b.company_id
		WHERE b.org_id = $1
		  AND bra.status = 'checked_in'
		ORDER BY b.booking_number, ro.name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var guests []InHouseGuest
	for rows.Next() {
		var g InHouseGuest
		var aid uuid.NullUUID
		if err := rows.Scan(&g.BookingID, &g.BookingNumber, &aid, &g.GuestName, &g.RoomName, &g.CompanyName); err != nil {
			return nil, err
		}
		if aid.Valid {
			g.AttendeeID = &aid.UUID
		}
		guests = append(guests, g)
	}
	return guests, rows.Err()
}

// AddItems appends more items to an existing order, snapshotting prices at the time of addition.
// Returns the updated order and the newly inserted items so the caller can append them to the invoice.
func (r *OrderRepository) AddItems(orderID uuid.UUID, orgID uuid.UUID, items []models.PlaceOrderItemRequest) (*models.Order, []models.OrderItem, error) {
	var exists bool
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM orders WHERE id=$1 AND org_id=$2)`, orderID, orgID).Scan(&exists)
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, fmt.Errorf("order not found")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	now := time.Now()
	var newItems []models.OrderItem
	for _, req := range items {
		var price float64
		var itemName string
		err = tx.QueryRow(`SELECT price, name FROM menu_items WHERE id=$1 AND org_id=$2`, req.MenuItemID, orgID).Scan(&price, &itemName)
		if err != nil {
			return nil, nil, fmt.Errorf("menu item %s not found", req.MenuItemID)
		}
		subtotal := price * float64(req.Quantity)
		itemID := uuid.New()
		_, err = tx.Exec(`
			INSERT INTO order_items (id, order_id, menu_item_id, quantity, unit_price, subtotal, notes, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			itemID, orderID, req.MenuItemID, req.Quantity, price, subtotal, req.Notes, now,
		)
		if err != nil {
			return nil, nil, err
		}
		newItems = append(newItems, models.OrderItem{
			ID:         itemID,
			OrderID:    orderID,
			MenuItemID: req.MenuItemID,
			ItemName:   itemName,
			Quantity:   req.Quantity,
			UnitPrice:  price,
			Subtotal:   subtotal,
			Notes:      req.Notes,
			CreatedAt:  now,
		})
	}

	_, err = tx.Exec(`UPDATE orders SET updated_at=$1 WHERE id=$2`, now, orderID)
	if err != nil {
		return nil, nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	order, err := r.GetByID(orderID, orgID)
	return order, newItems, err
}

// CloseOrdersForDay closes all open orders created on or before today for a given org.
// Returns the number of orders closed.
func (r *OrderRepository) CloseOrdersForDay(orgID uuid.UUID) (int64, error) {
	res, err := r.db.Exec(`
		UPDATE orders
		SET status=$1, updated_at=$2
		WHERE org_id=$3
		  AND status=$4
		  AND created_at::date <= CURRENT_DATE`,
		models.OrderStatusClosed, time.Now(), orgID, models.OrderStatusOpen,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RemoveItem deletes a single item from an open order.
// Returns the deleted item's subtotal so the caller can remove the matching invoice line item.
func (r *OrderRepository) RemoveItem(itemID uuid.UUID, orderID uuid.UUID, orgID uuid.UUID) (float64, error) {
	// Confirm order exists, belongs to org, and is still open
	var status string
	err := r.db.QueryRow(`SELECT status FROM orders WHERE id=$1 AND org_id=$2`, orderID, orgID).Scan(&status)
	if err != nil {
		return 0, fmt.Errorf("order not found")
	}
	if status != models.OrderStatusOpen {
		return 0, fmt.Errorf("cannot remove items from a closed order")
	}

	var subtotal float64
	err = r.db.QueryRow(`SELECT subtotal FROM order_items WHERE id=$1 AND order_id=$2`, itemID, orderID).Scan(&subtotal)
	if err != nil {
		return 0, fmt.Errorf("order item not found")
	}

	_, err = r.db.Exec(`DELETE FROM order_items WHERE id=$1 AND order_id=$2`, itemID, orderID)
	if err != nil {
		return 0, err
	}

	_, _ = r.db.Exec(`UPDATE orders SET updated_at=$1 WHERE id=$2`, time.Now(), orderID)
	return subtotal, nil
}

// GetItemsTotal returns the sum of all item subtotals for an order.
func (r *OrderRepository) GetItemsTotal(orderID uuid.UUID) (float64, error) {
	var total float64
	err := r.db.QueryRow(`SELECT COALESCE(SUM(subtotal), 0) FROM order_items WHERE order_id=$1`, orderID).Scan(&total)
	return total, err
}

// ListItemsByBookingID returns every order item across all of a booking's orders,
// regardless of order status. Used by invoicing to bill a meals booking. The
// attendee name (when the order is tied to a guest) is returned in item.Notes-free
// ItemName-adjacent field via the joined attendee_name for line descriptions.
func (r *OrderRepository) ListItemsByBookingID(bookingID, orgID uuid.UUID) ([]models.OrderItem, []string, error) {
	rows, err := r.db.Query(`
		SELECT oi.id, oi.order_id, oi.menu_item_id, oi.attendee_id,
		       COALESCE(att.full_name, '') AS attendee_name,
		       mi.name, oi.quantity, oi.unit_price, oi.subtotal, oi.notes, oi.created_at
		FROM order_items oi
		JOIN orders     o   ON o.id  = oi.order_id
		JOIN menu_items mi  ON mi.id = oi.menu_item_id
		LEFT JOIN booking_attendees att ON att.id = oi.attendee_id
		WHERE o.booking_id = $1 AND o.org_id = $2
		ORDER BY att.full_name ASC NULLS LAST, oi.created_at ASC`, bookingID, orgID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var items []models.OrderItem
	var attendeeNames []string
	for rows.Next() {
		var item models.OrderItem
		var attendeeID uuid.NullUUID
		var attendeeName, notes sql.NullString
		if err := rows.Scan(&item.ID, &item.OrderID, &item.MenuItemID, &attendeeID, &attendeeName,
			&item.ItemName, &item.Quantity, &item.UnitPrice, &item.Subtotal, &notes, &item.CreatedAt); err != nil {
			return nil, nil, err
		}
		if attendeeID.Valid {
			item.AttendeeID = &attendeeID.UUID
		}
		if attendeeName.Valid {
			item.AttendeeName = attendeeName.String
		}
		if notes.Valid {
			item.Notes = notes.String
		}
		items = append(items, item)
		attendeeNames = append(attendeeNames, item.AttendeeName)
	}
	return items, attendeeNames, rows.Err()
}

func (r *OrderRepository) fetchItems(orderID uuid.UUID) ([]models.OrderItem, error) {
	rows, err := r.db.Query(`
		SELECT oi.id, oi.order_id, oi.menu_item_id, oi.attendee_id,
		       COALESCE(att.full_name, '') AS attendee_name,
		       mi.name, oi.quantity, oi.unit_price, oi.subtotal, oi.notes, oi.created_at
		FROM order_items oi
		JOIN menu_items mi ON mi.id = oi.menu_item_id
		LEFT JOIN booking_attendees att ON att.id = oi.attendee_id
		WHERE oi.order_id=$1
		ORDER BY att.full_name ASC NULLS LAST, oi.created_at ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.OrderItem
	for rows.Next() {
		var item models.OrderItem
		var attendeeID uuid.NullUUID
		var attendeeName, notes sql.NullString
		if err := rows.Scan(&item.ID, &item.OrderID, &item.MenuItemID, &attendeeID, &attendeeName,
			&item.ItemName, &item.Quantity, &item.UnitPrice, &item.Subtotal, &notes, &item.CreatedAt); err != nil {
			return nil, err
		}
		if attendeeID.Valid {
			item.AttendeeID = &attendeeID.UUID
		}
		if attendeeName.Valid {
			item.AttendeeName = attendeeName.String
		}
		if notes.Valid {
			item.Notes = notes.String
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
