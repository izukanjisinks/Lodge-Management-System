package jobs

import (
	"context"
	"log"
	"time"

	"lodge-system/internal/repository"
	"lodge-system/internal/services"
)

// CloseOrdersJob runs nightly and closes all open orders from the current day
// for each organization that has auto_close_orders enabled.
type CloseOrdersJob struct {
	orderSvc     *services.OrderService
	settingsRepo *repository.OrganizationSettingsRepository
}

func NewCloseOrdersJob(
	orderSvc *services.OrderService,
	settingsRepo *repository.OrganizationSettingsRepository,
) *CloseOrdersJob {
	return &CloseOrdersJob{orderSvc: orderSvc, settingsRepo: settingsRepo}
}

// Start launches the job in a background goroutine, firing once at the next midnight UTC
// and then every 24 hours thereafter.
func (j *CloseOrdersJob) Start() {
	go func() {
		time.Sleep(durationUntilMidnight())
		j.run()

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			j.run()
		}
	}()
}

func (j *CloseOrdersJob) run() {
	log.Println("[close-orders] running nightly order close")

	orgIDs, err := j.settingsRepo.ListEnabledOrgsForJob("auto_close_orders")
	if err != nil {
		log.Printf("[close-orders] failed to fetch enabled orgs: %v", err)
		return
	}

	totalClosed := int64(0)
	for _, orgID := range orgIDs {
		n, err := j.orderSvc.CloseAllOrders(context.Background(), orgID)
		if err != nil {
			log.Printf("[close-orders] failed to close orders for org %s: %v", orgID, err)
			continue
		}
		totalClosed += n
	}

	log.Printf("[close-orders] closed %d open order(s) across %d org(s)", totalClosed, len(orgIDs))
}
