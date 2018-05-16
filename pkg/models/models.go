package models

import (
	"regexp"
	"github.com/emphant/redis-mulive-router/pkg/utils/errors"
)

func ValidateProduct(name string) error {
	if regexp.MustCompile(`^\w[\w\.\-]*$`).MatchString(name) {
		return nil
	}
	return errors.Errorf("bad product name = %s", name)
}
