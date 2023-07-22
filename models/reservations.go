package models

import (
	"time"

	"gorm.io/gorm"
)

type Reservation struct {
	gorm.Model
	ID             int64             `json:"id"`
	AccomodationId int64             `json:"accomodation_id"`
	NumberOfGuests int64             `json:"number_of_guests"`
	UserId         int64             `json:"user_id"`
	StartDate      time.Time         `json:"start_date"`
	EndDate        time.Time         `json:"end_date"`
	Status         ReservationStatus `json:"status"`
}

type ReservationStatus int32

const (
	UNSPECIFIED ReservationStatus = 0
	ACTIVE      ReservationStatus = 1
	ACCEPTED    ReservationStatus = 2
	DECLINED    ReservationStatus = 3
)
