// File: /database/database.go
package database

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"motocosmos-api/models"
)

func Initialize(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(databaseURL), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Motorcycle{},
		&models.Route{},
		&models.RouteWaypoint{},
		&models.CommunityEvent{},
		&models.EventParticipant{},
		&models.Post{},
		&models.PostLike{},
		&models.Follow{},
		&models.RideRecord{},
		&models.RoutePoint{},
		&models.UserLocation{},
		&models.SavedRoute{},
		&models.TripCalculation{},
	)
}
