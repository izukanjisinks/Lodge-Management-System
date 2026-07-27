package services

import (
	"context"
	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type DashboardService struct {
	repo *repository.DashboardRepository
}

func NewDashboardService(repo *repository.DashboardRepository) *DashboardService {
	return &DashboardService{repo: repo}
}

// GetSummary backs the always-loaded top 3 stat cards.
func (s *DashboardService) GetSummary(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) (*models.DashboardSummary, error) {
	summary, err := s.repo.Summary(orgID, branchID)
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

// GetBookings backs the Bookings tab (the default tab).
func (s *DashboardService) GetBookings(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) (*models.DashboardBookings, error) {
	overstaying, err := s.repo.OverstayingGuests(orgID, branchID)
	if err != nil {
		return nil, err
	}
	pendingApprovals, err := s.repo.PendingApprovals(orgID, branchID)
	if err != nil {
		return nil, err
	}
	revenueByMonth, err := s.repo.RevenueByMonth(orgID, branchID, 12)
	if err != nil {
		return nil, err
	}
	reservations, err := s.repo.ReservationsBreakdown(orgID, branchID, 30)
	if err != nil {
		return nil, err
	}
	recentBookings, err := s.repo.RecentBookings(orgID, branchID, 5)
	if err != nil {
		return nil, err
	}

	return &models.DashboardBookings{
		OverstayingGuests:     overstaying,
		PendingApprovals:      pendingApprovals,
		RevenueByMonth:        revenueByMonth,
		ReservationsBreakdown: reservations,
		RecentBookings:        recentBookings,
	}, nil
}

// GetOrders backs the Orders tab.
func (s *DashboardService) GetOrders(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) (*models.DashboardOrders, error) {
	kitchenBacklog, err := s.repo.KitchenBacklog(orgID, branchID)
	if err != nil {
		return nil, err
	}
	barBacklog, err := s.repo.BarBacklog(orgID, branchID)
	if err != nil {
		return nil, err
	}
	volumeByDay, err := s.repo.OrderVolumeByDay(orgID, branchID, 7)
	if err != nil {
		return nil, err
	}
	byStation, err := s.repo.OrdersByStation(orgID, branchID)
	if err != nil {
		return nil, err
	}
	recentOrders, err := s.repo.RecentOrders(orgID, branchID, 5)
	if err != nil {
		return nil, err
	}

	return &models.DashboardOrders{
		KitchenBacklog: kitchenBacklog,
		BarBacklog:     barBacklog,
		VolumeByDay:    volumeByDay,
		ByStation:      byStation,
		RecentOrders:   recentOrders,
	}, nil
}

// GetInvoices backs the Invoices tab.
func (s *DashboardService) GetInvoices(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) (*models.DashboardInvoices, error) {
	invoices, err := s.repo.InvoicesSummary(orgID, branchID)
	if err != nil {
		return nil, err
	}
	billedVsCollected, err := s.repo.BilledVsCollected(orgID, branchID, 6)
	if err != nil {
		return nil, err
	}
	byStatus, err := s.repo.InvoicesByStatus(orgID, branchID)
	if err != nil {
		return nil, err
	}
	outstanding, err := s.repo.OutstandingInvoices(orgID, branchID, 5)
	if err != nil {
		return nil, err
	}

	invoices.BilledVsCollected = billedVsCollected
	invoices.ByStatus = byStatus
	invoices.OutstandingInvoices = outstanding
	return &invoices, nil
}
