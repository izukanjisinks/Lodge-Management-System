// Package interfaces declares the service contracts that HTTP handlers depend on.
// Handlers take these interfaces (not concrete services), which lets us wrap a
// service in cross-cutting decorators (logging, telemetry) transparently.
//
// Convention: context.Context is the first argument of every method so that a
// request-scoped trace span travels down the call chain to the telemetry decorator.
package interfaces

import (
	"context"
	"time"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

// OrderInterface is the contract implemented by *services.OrderService and by the
// logging / telemetry decorators that wrap it. It covers exactly the methods the
// order HTTP handler consumes.
type OrderInterface interface {
	List(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, orderType, status string, bookingID *uuid.UUID, from, to *time.Time, page, pageSize int) ([]models.Order, int, error)
	ListCheckedInGuests(ctx context.Context, orgID uuid.UUID) ([]repository.InHouseGuest, error)
	GetByID(ctx context.Context, id, orgID uuid.UUID) (*models.Order, error)
	PlaceOrder(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, req *models.PlaceOrderRequest) (*models.Order, error)
	PlaceWalkInOrder(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, req *models.PlaceWalkInOrderRequest) (*models.Order, error)
	CloseAllOrders(ctx context.Context, orgID uuid.UUID) (int64, error)
	RemoveItem(ctx context.Context, itemID, orderID, orgID uuid.UUID) error
	UpdateKitchenStatus(ctx context.Context, id, orgID uuid.UUID, status string) (*models.Order, error)
	UpdateBarStatus(ctx context.Context, id, orgID uuid.UUID, status string) (*models.Order, error)
	AddItems(ctx context.Context, orderID, orgID uuid.UUID, req *models.AddOrderItemsRequest) (*models.Order, error)
}
