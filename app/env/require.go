package env

import "errors"

func Require(names []string) error {
	for _, n := range names {
		v := GetString(n)
		if len(v) == 0 {
			return errors.New("required environment variable is missing: " + n)
		}
	}
	return nil
}
