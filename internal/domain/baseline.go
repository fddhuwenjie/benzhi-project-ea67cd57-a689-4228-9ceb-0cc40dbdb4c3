package domain

import (
	"encoding/hex"
	"fmt"
	"strings"
)

func ValidateBaseline(j RegistrationJob) error {
	if strings.TrimSpace(j.Title) == "" || j.MapYear < 1000 || j.MapYear > 3000 || j.ImageWidth < 1 || j.ImageHeight < 1 || j.ImageWidth > 1000000 || j.ImageHeight > 1000000 || strings.TrimSpace(j.TargetCRS) == "" || j.RMSELimit <= 0 || j.RMSELimit > 1000000 || strings.TrimSpace(j.OperatorID) == "" {
		return fmt.Errorf("%w: invalid baseline", ErrRule)
	}
	b, e := hex.DecodeString(j.ImageSHA256)
	if e != nil || len(b) != 32 {
		return fmt.Errorf("%w: image_sha256", ErrRule)
	}
	return nil
}
