package models

import (
	"gorm.io/gorm"
	"time"
)

type Availability struct {
	gorm.Model
	ID              int64     `json:"id"`
	AccommodationId int64     `json:"accommodation_id"`
	Price           int64     `json:"price"`
	StartDate       time.Time `json:"start_date"       validate:"gt=time.Now()"`
	EndDate         time.Time `json:"end_date"         validate:"gtfield=StartDate"`
}
