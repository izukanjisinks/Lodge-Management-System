package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type OrderService struct {
	repo     *repository.OrderRepository
	invoice  *repository.InvoiceRepository
	booking  *repository.BookingRepository
	auditLog *repository.AuditLogRepository
}

func NewOrderService(
	repo *repository.OrderRepository,
	invoice *repository.InvoiceRepository,
	booking *repository.BookingRepository,
	auditLog *repository.AuditLogRepository,
) *OrderService {
	return &OrderService{repo: repo, invoice: invoice, booking: booking, auditLog: auditLog}
}

// PlaceOrder creates a new in-house order tied to a confirmed/checked-in booking
// and immediately appends a line item to the booking's invoice.
func (s *OrderService) PlaceOrder(orgID uuid.UUID, branchID *uuid.UUID, req *models.PlaceOrderRequest) (*models.Order, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("at least one item is required")
	}

	b, err := s.booking.GetByID(req.BookingID, orgID)
	if err != nil {
		return nil, errors.New("booking not found")
	}
	if b.Status != models.BookingStatusConfirmed && b.Status != models.BookingStatusCheckedIn {
		return nil, fmt.Errorf("orders can only be placed on confirmed or checked-in bookings (current status: %s)", b.Status)
	}

	o := &models.Order{
		BranchID:   b.BranchID,
		BookingID:  &req.BookingID,
		AttendeeID: req.AttendeeID,
		Type:       models.OrderTypeInHouse,
		Notes:      req.Notes,
	}
	// Fall back to the caller's branch if the booking has none
	if o.BranchID == nil {
		o.BranchID = branchID
	}
	order, err := s.repo.Create(o, req.Items, orgID)
	if err != nil {
		return nil, err
	}

	s.appendToInvoice(order, orgID, order.Items)
	return order, nil
}

// PlaceWalkInOrder creates a new walk-in order with no booking — no invoice entry.
func (s *OrderService) PlaceWalkInOrder(orgID uuid.UUID, branchID *uuid.UUID, req *models.PlaceWalkInOrderRequest) (*models.Order, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("at least one item is required")
	}
	if branchID == nil {
		return nil, errors.New("branch is required")
	}
	o := &models.Order{
		BranchID: branchID,
		Type:     models.OrderTypeWalkIn,
		Notes:    req.Notes,
	}
	return s.repo.Create(o, req.Items, orgID)
}

// AddItems appends more items to an existing order and updates the invoice immediately.
func (s *OrderService) AddItems(orderID uuid.UUID, orgID uuid.UUID, req *models.AddOrderItemsRequest) (*models.Order, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("at least one item is required")
	}
	order, newItems, err := s.repo.AddItems(orderID, orgID, req.Items)
	if err != nil {
		return nil, err
	}

	s.appendToInvoice(order, orgID, newItems)
	return order, nil
}

// RemoveItem removes a single item from an open order and removes its invoice line item if one exists.
func (s *OrderService) RemoveItem(itemID uuid.UUID, orderID uuid.UUID, orgID uuid.UUID) error {
	order, err := s.repo.GetByID(orderID, orgID)
	if err != nil {
		return errors.New("order not found")
	}

	if _, err := s.repo.RemoveItem(itemID, orderID, orgID); err != nil {
		return err
	}

	if order.BookingID != nil {
		if invErr := s.invoice.RemoveOrderLineItem(*order.BookingID, orgID, itemID); invErr != nil {
			fmt.Printf("warning: failed to remove invoice line item for order item %s: %v\n", itemID, invErr)
		}
	}
	return nil
}

// UpdateKitchenStatus advances the kitchen status of an open order.
func (s *OrderService) UpdateKitchenStatus(id uuid.UUID, orgID uuid.UUID, status string) (*models.Order, error) {
	switch status {
	case models.KitchenStatusNew, models.KitchenStatusPreparing, models.KitchenStatusReady:
	default:
		return nil, fmt.Errorf("invalid kitchen_status %q: must be new, preparing, or ready", status)
	}
	return s.repo.UpdateKitchenStatus(id, orgID, status)
}

// UpdateBarStatus advances the bar status of an open order. Independent of
// kitchen_status — an order can carry both statuses at once.
func (s *OrderService) UpdateBarStatus(id uuid.UUID, orgID uuid.UUID, status string) (*models.Order, error) {
	switch status {
	case models.BarStatusNew, models.BarStatusPreparing, models.BarStatusReady:
	default:
		return nil, fmt.Errorf("invalid bar_status %q: must be new, preparing, or ready", status)
	}
	return s.repo.UpdateBarStatus(id, orgID, status)
}

// CloseAllOrders closes every open order for the org. Returns the count closed.
func (s *OrderService) CloseAllOrders(orgID uuid.UUID) (int64, error) {
	count, err := s.repo.CloseOrdersForDay(orgID)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		s.writeOrdersClosedAuditLog(orgID, count)
	}
	return count, nil
}

func (s *OrderService) writeOrdersClosedAuditLog(orgID uuid.UUID, count int64) {
	payload := models.OrdersClosedPayload{
		OrdersClosed: count,
		ClosedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[order-service] failed to marshal audit payload for orders.closed: %v", err)
		return
	}
	entry := &models.AuditLog{
		OrgID:      orgID,
		ActorType:  models.AuditActorSystem,
		ActorName:  "order-service",
		Action:     models.AuditActionOrdersClosed,
		EntityType: models.AuditEntityOrder,
		EntityID:   orgID,
		Payload:    raw,
	}
	if err := s.auditLog.Insert(entry); err != nil {
		log.Printf("[order-service] failed to write audit log for orders.closed (org %s): %v", orgID, err)
	}
}

func (s *OrderService) ListCheckedInGuests(orgID uuid.UUID) ([]repository.InHouseGuest, error) {
	return s.repo.ListCheckedInGuests(orgID)
}

func (s *OrderService) GetByID(id uuid.UUID, orgID uuid.UUID) (*models.Order, error) {
	o, err := s.repo.GetByID(id, orgID)
	if err != nil {
		return nil, errors.New("order not found")
	}
	return o, nil
}

func (s *OrderService) List(orgID uuid.UUID, branchID *uuid.UUID, orderType, status string, bookingID *uuid.UUID, from, to *time.Time, page, pageSize int) ([]models.Order, int, error) {
	return s.repo.List(orgID, branchID, orderType, status, bookingID, from, to, page, pageSize)
}

// appendToInvoice writes one invoice line item per order item onto the booking's invoice.
// For corporate bookings the description is prefixed with the booker name.
// Non-fatal — logs on failure so the order itself is never rolled back.
func (s *OrderService) appendToInvoice(order *models.Order, orgID uuid.UUID, items []models.OrderItem) {
	if order.BookingID == nil || len(items) == 0 {
		return
	}

	b, err := s.booking.GetByID(*order.BookingID, orgID)
	if err != nil {
		return
	}

	var inv *models.Invoice
	inv, err = s.invoice.GetByBookingID(*order.BookingID, orgID)
	if err != nil {
		return
	}

	orderID := order.ID
	bookingID := *order.BookingID
	isCorporate := b.BookerType == models.BookerTypeCorporate
	guestLabel := b.BookerName
	if order.AttendeeName != "" {
		guestLabel = order.AttendeeName
	}
	for _, item := range items {
		itemID := item.ID
		var description string
		if isCorporate {
			description = fmt.Sprintf("%s — %s × %d", guestLabel, item.ItemName, item.Quantity)
		} else {
			description = fmt.Sprintf("%s × %d", item.ItemName, item.Quantity)
		}
		lineItem := &models.InvoiceLineItem{
			BookingID:   &bookingID,
			OrderID:     &orderID,
			OrderItemID: &itemID,
			Description: description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			Total:       item.Subtotal,
		}
		if err := s.invoice.AppendOrderLineItem(inv.ID, orgID, lineItem); err != nil {
			fmt.Printf("warning: failed to append order item %s to invoice %s: %v\n", item.ID, inv.ID, err)
		}
	}
}
