// File: /jobs/location_cleanup_job.go
package jobs

import (
	"fmt"
	"gorm.io/gorm"
	"motocosmos-api/repositories"
	"motocosmos-api/services"
	"time"
)

// LocationCleanupJob handles periodic cleanup of inactive user locations
type LocationCleanupJob struct {
	db              *gorm.DB
	locationService *services.LocationService
	ticker          *time.Ticker
	done            chan bool
}

// NewLocationCleanupJob creates a new location cleanup job
func NewLocationCleanupJob(db *gorm.DB, interval time.Duration) *LocationCleanupJob {
	locationRepo := repositories.NewLocationRepository(db)
	locationService := services.NewLocationService(locationRepo)

	return &LocationCleanupJob{
		db:              db,
		locationService: locationService,
		ticker:          time.NewTicker(interval),
		done:            make(chan bool),
	}
}

// Start begins the cleanup job
func (j *LocationCleanupJob) Start() {
	fmt.Println("Location cleanup job started")

	go func() {
		// Run immediately on start
		j.cleanup()

		// Then run on schedule
		for {
			select {
			case <-j.ticker.C:
				j.cleanup()
			case <-j.done:
				fmt.Println("Location cleanup job stopped")
				return
			}
		}
	}()
}

// Stop stops the cleanup job
func (j *LocationCleanupJob) Stop() {
	j.ticker.Stop()
	j.done <- true
}

// cleanup performs the actual cleanup
func (j *LocationCleanupJob) cleanup() {
	fmt.Println("Running location cleanup...")

	err := j.locationService.CleanupInactiveLocations()
	if err != nil {
		fmt.Printf("Error during location cleanup: %v\n", err)
		return
	}

	fmt.Println("Location cleanup completed successfully")
}
