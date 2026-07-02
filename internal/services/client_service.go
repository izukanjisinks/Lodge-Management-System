package services

import (
	"errors"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

type ClientService struct {
	repo            *repository.ClientRepository
	bookingRepo     *repository.BookingRepository
	bookingDocRepo  *repository.BookingDocumentRepository
}

func NewClientService(repo *repository.ClientRepository) *ClientService {
	return &ClientService{repo: repo}
}

func (s *ClientService) SetBookingRepository(bookingRepo *repository.BookingRepository) {
	s.bookingRepo = bookingRepo
}

func (s *ClientService) SetBookingDocumentRepository(r *repository.BookingDocumentRepository) {
	s.bookingDocRepo = r
}

// ─── Individual ───────────────────────────────────────────────────────────────

func (s *ClientService) CreateIndividual(orgID uuid.UUID, c *models.IndividualClient) error {
	if c.FullName == "" {
		return errors.New("full_name is required")
	}
	if c.Email == "" {
		return errors.New("email is required")
	}
	if c.Phone == "" {
		return errors.New("phone is required")
	}
	if c.IDPassportNumber == "" {
		return errors.New("id_passport_number is required")
	}
	if c.Status == "" {
		c.Status = models.ClientStatusActive
	}
	if err := s.repo.CreateIndividual(c, orgID); err != nil {
		return formatConstraintError(err)
	}
	return nil
}

func (s *ClientService) GetIndividualByID(id uuid.UUID, orgID uuid.UUID) (*models.IndividualClient, error) {
	return s.repo.GetIndividualByID(id, orgID)
}

func (s *ClientService) ListIndividual(orgID uuid.UUID, search, status string, page, pageSize int) ([]models.IndividualClient, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return s.repo.ListIndividual(orgID, search, status, page, pageSize)
}

func (s *ClientService) UpdateIndividual(id uuid.UUID, orgID uuid.UUID, updates *models.IndividualClient) (*models.IndividualClient, error) {
	existing, err := s.repo.GetIndividualByID(id, orgID)
	if err != nil {
		return nil, errors.New("individual client not found")
	}
	if updates.FullName != "" {
		existing.FullName = updates.FullName
	}
	if updates.Email != "" {
		existing.Email = updates.Email
	}
	if updates.Phone != "" {
		existing.Phone = updates.Phone
	}
	if updates.IDPassportNumber != "" {
		existing.IDPassportNumber = updates.IDPassportNumber
	}
	if updates.Nationality != "" {
		existing.Nationality = updates.Nationality
	}
	if updates.Status != "" {
		existing.Status = updates.Status
	}
	existing.Notes = updates.Notes
	if err := s.repo.UpdateIndividual(existing, orgID); err != nil {
		return nil, err
	}
	return s.repo.GetIndividualByID(id, orgID)
}

func (s *ClientService) DeleteIndividual(id uuid.UUID, orgID uuid.UUID) error {
	return s.repo.DeleteIndividual(id, orgID)
}

func (s *ClientService) LookupIndividualByIDNumber(orgID uuid.UUID, idNumber string) (*models.IndividualClient, error) {
	if idNumber == "" {
		return nil, errors.New("id_number is required")
	}
	return s.repo.LookupIndividualByIDNumber(orgID, idNumber)
}

// ─── Corporate ────────────────────────────────────────────────────────────────

// CreateCorporate registers a company in cor_company_details. Company name plus
// a reg number are required — reg number is part of the dedup key. Contact fields
// (person/email/phone) are captured on cor_profiles through the booking chain, not
// here, so they aren't required for a staff-created company shell.
func (s *ClientService) CreateCorporate(orgID uuid.UUID, c *models.CorporateClient) error {
	if c.CompanyName == "" {
		return errors.New("company_name is required")
	}
	if c.CompanyRegNumber == "" {
		return errors.New("company_reg_number is required")
	}
	if c.Status == "" {
		c.Status = models.ClientStatusActive
	}
	if err := s.repo.CreateCorporate(c, orgID); err != nil {
		return formatConstraintError(err)
	}
	return nil
}

func (s *ClientService) GetCorporateByID(id uuid.UUID, orgID uuid.UUID) (*models.CorporateClient, error) {
	return s.repo.GetCorporateByID(id, orgID)
}

func (s *ClientService) ListCorporate(orgID uuid.UUID, search, status string, page, pageSize int) ([]models.CorporateClient, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return s.repo.ListCorporate(orgID, search, status, page, pageSize)
}

// UpdateCorporate updates the company-level fields on cor_company_details. Contact
// person/email/phone are read-only here (they belong to cor_profiles), so edits to
// them from the client dialog are ignored.
func (s *ClientService) UpdateCorporate(id uuid.UUID, orgID uuid.UUID, updates *models.CorporateClient) (*models.CorporateClient, error) {
	existing, err := s.repo.GetCorporateByID(id, orgID)
	if err != nil {
		return nil, errors.New("corporate client not found")
	}
	if updates.CompanyName != "" {
		existing.CompanyName = updates.CompanyName
	}
	if updates.CompanyRegNumber != "" {
		existing.CompanyRegNumber = updates.CompanyRegNumber
	}
	if updates.TPIN != "" {
		existing.TPIN = updates.TPIN
	}
	if updates.Industry != "" {
		existing.Industry = updates.Industry
	}
	if updates.Country != "" {
		existing.Country = updates.Country
	}
	if updates.Status != "" {
		existing.Status = updates.Status
	}
	if err := s.repo.UpdateCorporate(existing, orgID); err != nil {
		return nil, err
	}
	return s.repo.GetCorporateByID(id, orgID)
}

func (s *ClientService) DeleteCorporate(id uuid.UUID, orgID uuid.UUID) error {
	return s.repo.DeleteCorporate(id, orgID)
}

func (s *ClientService) SearchCorporate(orgID uuid.UUID, search string) ([]models.CorporateClient, error) {
	if search == "" {
		return []models.CorporateClient{}, nil
	}
	return s.repo.SearchCorporate(orgID, search, 10)
}

func (s *ClientService) GetCorporateWithBookings(id, orgID uuid.UUID) (*models.CorporateClientWithBookings, error) {
	client, err := s.repo.GetCorporateByID(id, orgID)
	if err != nil {
		return nil, errors.New("corporate client not found")
	}

	var guests []models.CorporateBookingGuest
	var documents []string

	if s.bookingRepo != nil {
		bookings, _, err := s.bookingRepo.List(orgID, models.BookerTypeCorporate, "", "", nil, nil, 1, 100)
		if err == nil {
			for _, b := range bookings {
				if b.CompanyID == nil || *b.CompanyID != id {
					continue
				}
				guests = append(guests, models.CorporateBookingGuest{
					BookingID:     b.ID.String(),
					BookingNumber: b.BookingNumber,
					ClientName:    b.BookerName,
					Status:        b.Status,
				})
			}
		}
	}
	if s.bookingDocRepo != nil {
		doc, err := s.bookingDocRepo.GetByCorporateClientID(id, orgID)
		if err == nil && doc != nil {
			documents = doc.URLs
		}
	}
	if guests == nil {
		guests = []models.CorporateBookingGuest{}
	}
	if documents == nil {
		documents = []string{}
	}

	return &models.CorporateClientWithBookings{
		CorporateClient: client,
		Documents:       documents,
		Guests:          guests,
	}, nil
}

func (s *ClientService) UpsertDocuments(corporateClientID, orgID uuid.UUID, urls []string) (*models.BookingDocument, error) {
	if s.bookingDocRepo == nil {
		return nil, errors.New("document storage not available")
	}
	return s.bookingDocRepo.Upsert(corporateClientID, orgID, urls)
}
