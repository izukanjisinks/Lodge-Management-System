package services

import (
	"errors"
	"time"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type VenueService struct {
	repo *repository.VenueRepository
}

func NewVenueService(repo *repository.VenueRepository) *VenueService {
	return &VenueService{repo: repo}
}

func (s *VenueService) Create(venue *models.Venue, orgID uuid.UUID) error {
	if venue.Name == "" {
		return errors.New("venue name is required")
	}
	if venue.BranchID == nil {
		return errors.New("branch is required")
	}
	if !models.ValidVenueTypes[venue.VenueType] {
		return errors.New("invalid venue type: must be conference_hall, event_space, boardroom, outdoor, or dining")
	}
	if venue.Capacity <= 0 {
		return errors.New("capacity must be greater than 0")
	}
	if venue.BaseRate < 0 {
		return errors.New("base rate cannot be negative")
	}
	if venue.RateType == "" {
		venue.RateType = models.VenueRateDaily
	}
	if !models.ValidVenueRateTypes[venue.RateType] {
		return errors.New("invalid rate type: must be hourly or daily")
	}
	if venue.Amenities == nil {
		venue.Amenities = []string{}
	}
	if venue.Images == nil {
		venue.Images = []string{}
	}
	venue.IsAvailable = true
	return s.repo.Create(venue, orgID)
}

func (s *VenueService) GetByID(id uuid.UUID, orgID uuid.UUID) (*models.Venue, error) {
	return s.repo.GetByID(id, orgID)
}

func (s *VenueService) GetByIDUnscoped(id uuid.UUID) (*models.Venue, error) {
	return s.repo.GetByIDUnscoped(id)
}

func (s *VenueService) GuestList(orgID uuid.UUID, branchID *uuid.UUID, venueType string, from, to *time.Time) ([]models.Venue, error) {
	if venueType != "" && !models.ValidVenueTypes[venueType] {
		return nil, errors.New("invalid venue type filter")
	}
	if err := validateDateRange(from, to); err != nil {
		return nil, err
	}
	return s.repo.GuestList(orgID, branchID, venueType, from, to)
}

func (s *VenueService) List(orgID uuid.UUID, branchID *uuid.UUID, venueType string, isAvailable *bool, from, to *time.Time, page, pageSize int) ([]models.Venue, int, error) {
	if venueType != "" && !models.ValidVenueTypes[venueType] {
		return nil, 0, errors.New("invalid venue type filter")
	}
	if err := validateDateRange(from, to); err != nil {
		return nil, 0, err
	}
	return s.repo.List(orgID, branchID, venueType, isAvailable, from, to, page, pageSize)
}

// validateDateRange rejects an inverted [from, to] window. Either bound may be nil.
func validateDateRange(from, to *time.Time) error {
	if from != nil && to != nil && to.Before(*from) {
		return errors.New("to date must not be before from date")
	}
	return nil
}

func (s *VenueService) Update(id uuid.UUID, orgID uuid.UUID, updates *models.Venue) (*models.Venue, error) {
	venue, err := s.repo.GetByID(id, orgID)
	if err != nil {
		return nil, errors.New("venue not found")
	}
	if updates.Name != "" {
		venue.Name = updates.Name
	}
	if updates.VenueType != "" {
		if !models.ValidVenueTypes[updates.VenueType] {
			return nil, errors.New("invalid venue type")
		}
		venue.VenueType = updates.VenueType
	}
	if updates.Capacity > 0 {
		venue.Capacity = updates.Capacity
	}
	if updates.AreaSqm > 0 {
		venue.AreaSqm = updates.AreaSqm
	}
	if updates.Floor != "" {
		venue.Floor = updates.Floor
	}
	if updates.BaseRate >= 0 {
		venue.BaseRate = updates.BaseRate
	}
	if updates.RateType != "" {
		if !models.ValidVenueRateTypes[updates.RateType] {
			return nil, errors.New("invalid rate type")
		}
		venue.RateType = updates.RateType
	}
	if updates.Amenities != nil {
		venue.Amenities = updates.Amenities
	}
	if updates.Notes != "" {
		venue.Notes = updates.Notes
	}
	venue.IsAvailable = updates.IsAvailable
	if err := s.repo.Update(venue, orgID); err != nil {
		return nil, err
	}
	return s.repo.GetByID(id, orgID)
}

func (s *VenueService) UpdateImages(id uuid.UUID, orgID uuid.UUID, images []string) (*models.Venue, error) {
	_, err := s.repo.GetByID(id, orgID)
	if err != nil {
		return nil, errors.New("venue not found")
	}
	if images == nil {
		images = []string{}
	}
	if err := s.repo.UpdateImages(id, orgID, images); err != nil {
		return nil, err
	}
	return s.repo.GetByID(id, orgID)
}

func (s *VenueService) SetAvailability(id uuid.UUID, orgID uuid.UUID, available bool) error {
	_, err := s.repo.GetByID(id, orgID)
	if err != nil {
		return errors.New("venue not found")
	}
	return s.repo.SetAvailability(id, orgID, available)
}

func (s *VenueService) Delete(id uuid.UUID, orgID uuid.UUID) error {
	return s.repo.Delete(id, orgID)
}
