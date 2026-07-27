package services

import (
	"errors"
	"fmt"
	"net"
	"time"

	"lodge-system/internal/repository"

	"github.com/google/uuid"
)

// PrinterService sends raw ESC/POS jobs to a branch's configured network
// receipt printer (e.g. Epson TM-T88VI listening on the standard raw-print
// port 9100). It has no queueing or retry logic — a failed dial/write simply
// returns an error for the caller to surface.
type PrinterService struct {
	branchRepo *repository.BranchRepository
}

func NewPrinterService(branchRepo *repository.BranchRepository) *PrinterService {
	return &PrinterService{branchRepo: branchRepo}
}

const dialTimeout = 5 * time.Second

// ESC/POS control bytes used below.
var (
	escInit    = []byte{0x1B, 0x40}       // ESC @ — initialize printer
	escAlignC  = []byte{0x1B, 0x61, 0x01} // ESC a 1 — center align
	escAlignL  = []byte{0x1B, 0x61, 0x00} // ESC a 0 — left align
	escBoldOn  = []byte{0x1B, 0x45, 0x01} // ESC E 1 — bold on
	escBoldOff = []byte{0x1B, 0x45, 0x00} // ESC E 0 — bold off
	gsCutFull  = []byte{0x1D, 0x56, 0x00} // GS V 0 — full cut
)

// buildTestReceipt renders the ESC/POS byte sequence for a short test receipt.
// Shared by the server-side dial path (TestPrint) and the Electron terminal
// path (BuildTestPrintJob), so the two never drift out of sync.
func buildTestReceipt(branchName, printerAddr, printerLabel string) []byte {
	var buf []byte
	buf = append(buf, escInit...)
	buf = append(buf, escAlignC...)
	buf = append(buf, escBoldOn...)
	buf = append(buf, []byte("TEST PRINT\n")...)
	buf = append(buf, escBoldOff...)
	buf = append(buf, []byte(printerLabel+"\n")...)
	buf = append(buf, escAlignL...)
	buf = append(buf, []byte("--------------------------------\n")...)
	buf = append(buf, []byte(fmt.Sprintf("Branch:  %s\n", branchName))...)
	buf = append(buf, []byte(fmt.Sprintf("Printer: %s\n", printerAddr))...)
	buf = append(buf, []byte(fmt.Sprintf("Time:    %s\n", time.Now().Format("2006-01-02 15:04:05")))...)
	buf = append(buf, []byte("--------------------------------\n")...)
	buf = append(buf, []byte("If you can read this, the\nconnection is working.\n\n\n")...)
	buf = append(buf, gsCutFull...)
	return buf
}

// TestPrint dials the given branch's configured printer and sends a short
// test receipt. Returns an error describing exactly what failed (branch has
// no printer configured, dial/connect failed, or the write failed) so the
// caller can show staff something actionable rather than a generic failure.
// Only reachable when the API server itself has a network path to the
// printer (e.g. same LAN); for a remote-hosted server, use the terminal
// app's local print path (BuildTestPrintJob) instead.
func (s *PrinterService) TestPrint(branchID, orgID uuid.UUID) error {
	branch, err := s.branchRepo.GetByID(branchID, orgID)
	if err != nil {
		return errors.New("branch not found")
	}
	if branch.PrinterIP == nil || *branch.PrinterIP == "" {
		return errors.New("no printer configured for this branch")
	}

	addr := fmt.Sprintf("%s:%d", *branch.PrinterIP, branch.PrinterPort)
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return fmt.Errorf("could not reach printer at %s: %w", addr, err)
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(dialTimeout))

	name := "Receipt Printer"
	if branch.PrinterName != nil && *branch.PrinterName != "" {
		name = *branch.PrinterName
	}

	buf := buildTestReceipt(branch.Name, addr, name)
	if _, err := conn.Write(buf); err != nil {
		return fmt.Errorf("connected but failed to send print job: %w", err)
	}
	return nil
}

// PrintJob is the payload the Electron terminal app needs to print locally:
// the printer's own network location plus the pre-rendered ESC/POS bytes.
// The terminal has no printer-formatting logic of its own — it only relays
// these bytes to a raw TCP socket, keeping all formatting server-side.
type PrintJob struct {
	IP   string
	Port int
	Data []byte
}

// BuildTestPrintJob resolves the branch's printer config and renders a test
// receipt, without attempting to dial it — for callers (the Electron
// terminal, via the API) that will do the actual TCP write themselves,
// because they have local network access the server may not.
func (s *PrinterService) BuildTestPrintJob(branchID, orgID uuid.UUID) (*PrintJob, error) {
	branch, err := s.branchRepo.GetByID(branchID, orgID)
	if err != nil {
		return nil, errors.New("branch not found")
	}
	if branch.PrinterIP == nil || *branch.PrinterIP == "" {
		return nil, errors.New("no printer configured for this branch")
	}

	addr := fmt.Sprintf("%s:%d", *branch.PrinterIP, branch.PrinterPort)
	name := "Receipt Printer"
	if branch.PrinterName != nil && *branch.PrinterName != "" {
		name = *branch.PrinterName
	}

	return &PrintJob{
		IP:   *branch.PrinterIP,
		Port: branch.PrinterPort,
		Data: buildTestReceipt(branch.Name, addr, name),
	}, nil
}
