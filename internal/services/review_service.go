package services

import (
	"context"
	"errors"
	"fmt"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type ReviewService struct {
	repo        *repository.ReviewRepository
	bookingRepo *repository.BookingRepository
}

func NewReviewService(
	repo *repository.ReviewRepository,
	bookingRepo *repository.BookingRepository,
) *ReviewService {
	return &ReviewService{repo: repo, bookingRepo: bookingRepo}
}

func (s *ReviewService) Submit(ctx context.Context, userID uuid.UUID, req *models.SubmitReviewRequest) (*models.Review, error) {
	if err := validateScores(req); err != nil {
		return nil, err
	}

	// Verify the booking belongs to this web/guest user and is checked_out
	booking, err := s.bookingRepo.GetByIDUnscoped(req.BookingID)
	if err != nil {
		return nil, errors.New("booking not found")
	}
	if booking.WebUserID == nil || *booking.WebUserID != userID {
		return nil, errors.New("forbidden")
	}
	if booking.Status != models.BookingStatusCheckedOut {
		return nil, fmt.Errorf("reviews can only be submitted after check-out (booking status is %q)", booking.Status)
	}

	// One review per booking
	exists, err := s.repo.ExistsByBookingID(req.BookingID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("a review has already been submitted for this booking")
	}

	review := &models.Review{
		BookingID:   req.BookingID,
		Facilities:  req.Facilities,
		Cleanliness: req.Cleanliness,
		Services:    req.Services,
		Comfort:     req.Comfort,
		Location:    req.Location,
		Comment:     req.Comment,
	}

	if err := s.repo.Create(review); err != nil {
		return nil, err
	}
	return review, nil
}

func (s *ReviewService) GetSummary(ctx context.Context) (*models.RatingSummary, error) {
	return s.repo.GetSummary()
}

func validateScores(req *models.SubmitReviewRequest) error {
	scores := map[string]float64{
		"facilities":  req.Facilities,
		"cleanliness": req.Cleanliness,
		"services":    req.Services,
		"comfort":     req.Comfort,
		"location":    req.Location,
	}
	for field, score := range scores {
		if score < 0 || score > 5 {
			return fmt.Errorf("%s score must be between 0 and 5", field)
		}
	}
	return nil
}
