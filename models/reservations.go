package models

import (
	"time"

	"gorm.io/gorm"
)

type Reservation struct {
	gorm.Model
	ID             int64     `json:"id"`
	AccomodationId int64     `json:"accomodation_id"`
	UserId         int64     `json:"user_id"`
	StartDate      time.Time `json:"start_date"`
	EndDate        time.Time `json:"end_date"`
	Status         int32     `json:"status"`
}
