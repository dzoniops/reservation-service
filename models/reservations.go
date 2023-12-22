package models

import (
	"time"

	"gorm.io/gorm"
)

type Reservation struct {
	gorm.Model
	ID              int64             `json:"id"`
	AccommodationId int64             `json:"accommodation_id"`
	NumberOfGuests  int64             `json:"number_of_guests"`
	UserId          int64             `json:"user_id"`
	StartDate       time.Time         `json:"start_date"       validate:"gt=time.Now()"`
	EndDate         time.Time         `json:"end_date"         validate:"gtfield=StartDate"`
	Status          ReservationStatus `json:"status"`
	HostId          int64             `json:"host_id"`
}

type ReservationStatus int32

const (
	UNSPECIFIED ReservationStatus = 0
	PENDING     ReservationStatus = 1
	ACCEPTED    ReservationStatus = 2
	DECLINED    ReservationStatus = 3
)

// AfterSave change all other reservation to declined
func (r *Reservation) AfterSave(tx *gorm.DB) (err error) {
	if r.Status == ACCEPTED {
		var reservations []Reservation
		tx.Where(
			"start_date < ? and end_date > ? and status = ? and accommodation_id = ?",
			r.EndDate, r.StartDate, PENDING, r.AccommodationId).
			Find(&reservations)
		for _, r := range reservations {
			tx.Model(&r).Update("status", DECLINED)
		}
	}
	return
}
