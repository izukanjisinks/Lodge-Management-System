// Package telemetry holds service-layer tracing decorators. Each decorator wraps
// a service interface and records an OpenTelemetry span event per call, using the
// span already present on the request context.
//
// NOTE: these are no-ops until an OTel TracerProvider is installed and a
// request-level tracing middleware opens a span per request (see docs/
// observability-interfaces-logging-telemetry.md §3). They compile and are ready;
// wiring the SDK "lights them up" without touching this file.
package telemetry

import (
	"context"
	"fmt"
	"time"

	"lodge-system/internal/interfaces"
	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

// OrderTelemetryMiddleware decorates an OrderInterface with span events.
type OrderTelemetryMiddleware struct {
	next interfaces.OrderInterface
}

// NewOrderTelemetryMiddleware wraps next with tracing and returns it as the same
// interface, so it composes transparently with other decorators.
func NewOrderTelemetryMiddleware(next interfaces.OrderInterface) interfaces.OrderInterface {
	return &OrderTelemetryMiddleware{next: next}
}

// event records a named span event on the active span (a no-op span if none).
func event(ctx context.Context, name string) {
	trace.SpanFromContext(ctx).AddEvent(name)
}

func (m *OrderTelemetryMiddleware) List(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, orderType, status string, bookingID *uuid.UUID, from, to *time.Time, page, pageSize int) ([]models.Order, int, error) {
	event(ctx, "OrderService.List")
	return m.next.List(ctx, orgID, branchID, orderType, status, bookingID, from, to, page, pageSize)
}

func (m *OrderTelemetryMiddleware) ListCheckedInGuests(ctx context.Context, orgID uuid.UUID) ([]repository.InHouseGuest, error) {
	event(ctx, "OrderService.ListCheckedInGuests")
	return m.next.ListCheckedInGuests(ctx, orgID)
}

func (m *OrderTelemetryMiddleware) GetByID(ctx context.Context, id, orgID uuid.UUID) (*models.Order, error) {
	event(ctx, fmt.Sprintf("OrderService.GetByID: %v", id))
	return m.next.GetByID(ctx, id, orgID)
}

func (m *OrderTelemetryMiddleware) PlaceOrder(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, req *models.PlaceOrderRequest) (*models.Order, error) {
	event(ctx, "OrderService.PlaceOrder")
	return m.next.PlaceOrder(ctx, orgID, branchID, req)
}

func (m *OrderTelemetryMiddleware) PlaceWalkInOrder(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, req *models.PlaceWalkInOrderRequest) (*models.Order, error) {
	event(ctx, "OrderService.PlaceWalkInOrder")
	return m.next.PlaceWalkInOrder(ctx, orgID, branchID, req)
}

func (m *OrderTelemetryMiddleware) CloseAllOrders(ctx context.Context, orgID uuid.UUID) (int64, error) {
	event(ctx, "OrderService.CloseAllOrders")
	return m.next.CloseAllOrders(ctx, orgID)
}

func (m *OrderTelemetryMiddleware) RemoveItem(ctx context.Context, itemID, orderID, orgID uuid.UUID) error {
	event(ctx, fmt.Sprintf("OrderService.RemoveItem: %v", itemID))
	return m.next.RemoveItem(ctx, itemID, orderID, orgID)
}

func (m *OrderTelemetryMiddleware) UpdateKitchenStatus(ctx context.Context, id, orgID uuid.UUID, status string) (*models.Order, error) {
	event(ctx, fmt.Sprintf("OrderService.UpdateKitchenStatus: %v -> %s", id, status))
	return m.next.UpdateKitchenStatus(ctx, id, orgID, status)
}

func (m *OrderTelemetryMiddleware) UpdateBarStatus(ctx context.Context, id, orgID uuid.UUID, status string) (*models.Order, error) {
	event(ctx, fmt.Sprintf("OrderService.UpdateBarStatus: %v -> %s", id, status))
	return m.next.UpdateBarStatus(ctx, id, orgID, status)
}

func (m *OrderTelemetryMiddleware) AddItems(ctx context.Context, orderID, orgID uuid.UUID, req *models.AddOrderItemsRequest) (*models.Order, error) {
	event(ctx, fmt.Sprintf("OrderService.AddItems: %v", orderID))
	return m.next.AddItems(ctx, orderID, orgID, req)
}
