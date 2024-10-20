package util

import (
	"fmt"
)

func StringifyPort(port int) string {
	return fmt.Sprintf(":%d", port)
}
