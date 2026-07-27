package services

import (
	"errors"
	"net"
	"strings"

	"lodge-system/internal/models"
	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

// validatePrinterIP rejects a non-empty string that isn't a valid IPv4/IPv6
// address, so a typo surfaces at save time rather than at print time.
func validatePrinterIP(ip string) error {
	if ip == "" {
		return nil
	}
	if net.ParseIP(ip) == nil {
		return errors.New("printer_ip is not a valid IP address")
	}
	return nil
}

type BranchService struct {
	repo *repository.BranchRepository
}

func NewBranchService(repo *repository.BranchRepository) *BranchService {
	return &BranchService{repo: repo}
}

func (s *BranchService) Create(orgID uuid.UUID, req *models.CreateBranchRequest) (*models.Branch, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("branch name is required")
	}
	if strings.TrimSpace(req.BranchCode) == "" {
		return nil, errors.New("branch_code is required")
	}
	b := &models.Branch{
		OrgID:      orgID,
		Name:       strings.TrimSpace(req.Name),
		BranchCode: strings.ToUpper(strings.TrimSpace(req.BranchCode)),
		IsActive:   true,
		IsMain:     false,
	}
	if v := strings.TrimSpace(req.StreetAddress); v != "" {
		b.StreetAddress = &v
	}
	if v := strings.TrimSpace(req.City); v != "" {
		b.City = &v
	}
	if v := strings.TrimSpace(req.Country); v != "" {
		b.Country = &v
	}
	if v := strings.TrimSpace(req.Location); v != "" {
		b.Location = &v
	}
	if v := strings.TrimSpace(req.Phone); v != "" {
		b.Phone = &v
	}
	if v := strings.TrimSpace(req.Email); v != "" {
		b.Email = &v
	}
	b.Parking = req.Parking
	b.Restaurant = req.Restaurant
	if req.CheckInTime != nil {
		b.CheckInTime = req.CheckInTime
	}
	if req.CheckOutTime != nil {
		b.CheckOutTime = req.CheckOutTime
	}
	if err := validatePrinterIP(req.PrinterIP); err != nil {
		return nil, err
	}
	if v := strings.TrimSpace(req.PrinterIP); v != "" {
		b.PrinterIP = &v
	}
	b.PrinterPort = req.PrinterPort
	if b.PrinterPort == 0 {
		b.PrinterPort = 9100 // Epson raw ESC/POS default
	}
	if v := strings.TrimSpace(req.PrinterName); v != "" {
		b.PrinterName = &v
	}
	if err := s.repo.Create(b); err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, errors.New("branch_code already exists for this organization")
		}
		return nil, err
	}
	return b, nil
}

func (s *BranchService) GetByID(id, orgID uuid.UUID) (*models.Branch, error) {
	b, err := s.repo.GetByID(id, orgID)
	if err != nil {
		return nil, errors.New("branch not found")
	}
	return b, nil
}

func (s *BranchService) List(orgID uuid.UUID) ([]models.Branch, error) {
	return s.repo.List(orgID)
}

func (s *BranchService) Update(id, orgID uuid.UUID, req *models.UpdateBranchRequest) (*models.Branch, error) {
	b, err := s.repo.GetByID(id, orgID)
	if err != nil {
		return nil, errors.New("branch not found")
	}
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, errors.New("branch name cannot be empty")
		}
		b.Name = strings.TrimSpace(*req.Name)
	}
	if req.BranchCode != nil {
		if strings.TrimSpace(*req.BranchCode) == "" {
			return nil, errors.New("branch_code cannot be empty")
		}
		b.BranchCode = strings.ToUpper(strings.TrimSpace(*req.BranchCode))
	}
	if req.StreetAddress != nil {
		v := strings.TrimSpace(*req.StreetAddress)
		b.StreetAddress = &v
	}
	if req.City != nil {
		v := strings.TrimSpace(*req.City)
		b.City = &v
	}
	if req.Country != nil {
		v := strings.TrimSpace(*req.Country)
		b.Country = &v
	}
	if req.Location != nil {
		v := strings.TrimSpace(*req.Location)
		b.Location = &v
	}
	if req.Phone != nil {
		v := strings.TrimSpace(*req.Phone)
		b.Phone = &v
	}
	if req.Email != nil {
		v := strings.TrimSpace(*req.Email)
		b.Email = &v
	}
	if req.IsActive != nil {
		b.IsActive = *req.IsActive
	}
	if req.IsMain != nil {
		b.IsMain = *req.IsMain
	}
	if req.Parking != nil {
		b.Parking = *req.Parking
	}
	if req.Restaurant != nil {
		b.Restaurant = *req.Restaurant
	}
	if req.CheckInTime != nil {
		b.CheckInTime = req.CheckInTime
	}
	if req.CheckOutTime != nil {
		b.CheckOutTime = req.CheckOutTime
	}
	if req.PrinterIP != nil {
		if err := validatePrinterIP(*req.PrinterIP); err != nil {
			return nil, err
		}
		v := strings.TrimSpace(*req.PrinterIP)
		if v == "" {
			b.PrinterIP = nil
		} else {
			b.PrinterIP = &v
		}
	}
	if req.PrinterPort != nil {
		b.PrinterPort = *req.PrinterPort
	}
	if req.PrinterName != nil {
		v := strings.TrimSpace(*req.PrinterName)
		if v == "" {
			b.PrinterName = nil
		} else {
			b.PrinterName = &v
		}
	}
	if err := s.repo.Update(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *BranchService) Delete(id, orgID uuid.UUID) error {
	return s.repo.Delete(id, orgID)
}
