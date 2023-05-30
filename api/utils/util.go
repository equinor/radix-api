package utils

import (
	"fmt"
	"os"
	"strconv"
)

func IsDebugMode() (bool, error) {
	isDebugModeStr, ok := os.LookupEnv("DEBUG_MODE")
	if ok {
		isDebugMode, err := strconv.ParseBool(isDebugModeStr)
		if err != nil {
			return false, fmt.Errorf("DEBUG_MODE is not a valid boolean: %s", err.Error())
		}
		return isDebugMode, nil
	}
	return false, nil
}
