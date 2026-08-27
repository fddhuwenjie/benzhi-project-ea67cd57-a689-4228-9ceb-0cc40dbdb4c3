package domain

import (
	"fmt"
	"strings"
)

func ValidateOperator(j RegistrationJob, actor string) error {
	if strings.TrimSpace(actor) == "" || actor != j.OperatorID {
		return fmt.Errorf("%w: operator identity mismatch", ErrRule)
	}
	return nil
}

func ValidateReviewer(j RegistrationJob, reviewer string) error {
	if strings.TrimSpace(reviewer) == "" || reviewer == j.OperatorID {
		return fmt.Errorf("%w: independent reviewer required", ErrRule)
	}
	return nil
}
