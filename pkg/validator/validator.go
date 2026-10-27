package validator

import (
	"fmt"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func Init() error {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return fmt.Errorf("")
	}

	validators := map[string]validator.Func{
		"yemen_phone": validateYemenPhone,
	}

	for tage, fun := range validators {
		if err := v.RegisterValidation(tage, fun); err != nil {
			return fmt.Errorf("")
		}

	}
	return nil
}
