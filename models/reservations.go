package models

import "gorm.io/gorm"

type Reservation struct {
	gorm.Model
	ID int64
}
