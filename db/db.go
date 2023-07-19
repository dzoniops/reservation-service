package db

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/dzoniops/reservation-service/models"
)

var DB *gorm.DB

func InitDB() {
	var err error

	pgConnString := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		os.Getenv("PGHOST"),
		os.Getenv("PGPORT"),
		os.Getenv("PGDATABASE"),
		os.Getenv("PGUSER"),
		os.Getenv("PGPASSWORD"),
	)
	DB, err = gorm.Open(postgres.Open(pgConnString), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	DB.AutoMigrate(&models.Reservation{})
}
