// File: /utils/validators.go
package utils

import (
	"regexp"
	"unicode"
)

func IsValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func IsValidPassword(password string) bool {
	if len(password) < 6 {
		return false
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	// At least 3 of 4 character types required
	count := 0
	if hasUpper {
		count++
	}
	if hasLower {
		count++
	}
	if hasNumber {
		count++
	}
	if hasSpecial {
		count++
	}

	return count >= 3
}

func IsValidCalculatorInput(roadLength, fuelPrice, fuelConsumption float64) bool {
	return roadLength > 0 && roadLength <= 10000 &&
		fuelPrice > 0 && fuelPrice <= 10 &&
		fuelConsumption > 0 && fuelConsumption <= 50
}

func IsValidLatitude(lat float64) bool {
	return lat >= -90 && lat <= 90
}

func IsValidLongitude(lng float64) bool {
	return lng >= -180 && lng <= 180
}
