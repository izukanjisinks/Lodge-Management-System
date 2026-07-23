// Package logger holds service-layer logging decorators. Each decorator wraps a
// service interface, times every call, and emits a structured zap log line —
// without the service itself knowing anything about logging.
//
// It uses zap.L() (the global logger), which main.go installs via
// zap.ReplaceGlobals, so these lines share the app's configured sink/format.
package logger

import (
	"context"
	"time"

	"lodge-system/internal/interfaces"
	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// OrderLoggerMiddleware decorates an OrderInterface with per-method timing +
// error logging.
type OrderLoggerMiddleware struct {
	next interfaces.OrderInterface
}

// NewOrderLoggerMiddleware wraps next with logging and returns it as the same
// interface, so it composes transparently with other decorators.
func NewOrderLoggerMiddleware(next interfaces.OrderInterface) interfaces.OrderInterface {
	return &OrderLoggerMiddleware{next: next}
}

// logDone emits one structured line: method, elapsed time, and error if any.
// Called via defer so it fires on every return path.
func logDone(method string, start time.Time, errp *error, fields ...zap.Field) {
	all := append(fields,
		zap.String("method", method),
		zap.Duration("took", time.Since(start)),
	)
	if errp != nil && *errp != nil {
		all = append(all, zap.Error(*errp))
		zap.L().Error("order service call failed", all...)
		return
	}
	zap.L().Info("order service call", all...)
}

func (m *OrderLoggerMiddleware) List(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, orderType, status string, bookingID *uuid.UUID, from, to *time.Time, page, pageSize int) (_ []models.Order, _ int, err error) {
	defer logDone("List", time.Now(), &err, zap.String("org_id", orgID.String()), zap.String("status", status))
	return m.next.List(ctx, orgID, branchID, orderType, status, bookingID, from, to, page, pageSize)
}

func (m *OrderLoggerMiddleware) ListCheckedInGuests(ctx context.Context, orgID uuid.UUID) (_ []repository.InHouseGuest, err error) {
	defer logDone("ListCheckedInGuests", time.Now(), &err, zap.String("org_id", orgID.String()))
	return m.next.ListCheckedInGuests(ctx, orgID)
}

func (m *OrderLoggerMiddleware) GetByID(ctx context.Context, id, orgID uuid.UUID) (_ *models.Order, err error) {
	defer logDone("GetByID", time.Now(), &err, zap.String("order_id", id.String()))
	return m.next.GetByID(ctx, id, orgID)
}

func (m *OrderLoggerMiddleware) PlaceOrder(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, req *models.PlaceOrderRequest) (_ *models.Order, err error) {
	defer logDone("PlaceOrder", time.Now(), &err, zap.String("org_id", orgID.String()))
	return m.next.PlaceOrder(ctx, orgID, branchID, req)
}

func (m *OrderLoggerMiddleware) PlaceWalkInOrder(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, req *models.PlaceWalkInOrderRequest) (_ *models.Order, err error) {
	defer logDone("PlaceWalkInOrder", time.Now(), &err, zap.String("org_id", orgID.String()))
	return m.next.PlaceWalkInOrder(ctx, orgID, branchID, req)
}

func (m *OrderLoggerMiddleware) CloseAllOrders(ctx context.Context, orgID uuid.UUID) (_ int64, err error) {
	defer logDone("CloseAllOrders", time.Now(), &err, zap.String("org_id", orgID.String()))
	return m.next.CloseAllOrders(ctx, orgID)
}

func (m *OrderLoggerMiddleware) RemoveItem(ctx context.Context, itemID, orderID, orgID uuid.UUID) (err error) {
	defer logDone("RemoveItem", time.Now(), &err, zap.String("order_id", orderID.String()), zap.String("item_id", itemID.String()))
	return m.next.RemoveItem(ctx, itemID, orderID, orgID)
}

func (m *OrderLoggerMiddleware) UpdateKitchenStatus(ctx context.Context, id, orgID uuid.UUID, status string) (_ *models.Order, err error) {
	defer logDone("UpdateKitchenStatus", time.Now(), &err, zap.String("order_id", id.String()), zap.String("kitchen_status", status))
	return m.next.UpdateKitchenStatus(ctx, id, orgID, status)
}

func (m *OrderLoggerMiddleware) UpdateBarStatus(ctx context.Context, id, orgID uuid.UUID, status string) (_ *models.Order, err error) {
	defer logDone("UpdateBarStatus", time.Now(), &err, zap.String("order_id", id.String()), zap.String("bar_status", status))
	return m.next.UpdateBarStatus(ctx, id, orgID, status)
}

func (m *OrderLoggerMiddleware) AddItems(ctx context.Context, orderID, orgID uuid.UUID, req *models.AddOrderItemsRequest) (_ *models.Order, err error) {
	defer logDone("AddItems", time.Now(), &err, zap.String("order_id", orderID.String()))
	return m.next.AddItems(ctx, orderID, orgID, req)
}
