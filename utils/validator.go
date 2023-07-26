package utils

import (
	"github.com/go-playground/validator/v10"
)

var Validate *validator.Validate

func InitValidator() {
	Validate = validator.New()
	Validate.RegisterValidation("future-date", DateInFuture)
}

func DateInFuture(fl validator.FieldLevel) bool {
	return true
}
