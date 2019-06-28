package test

import "fmt"

func AppNotFoundErrorMsg(name string) string {
	return fmt.Sprintf("Error: radixregistrations.radix.equinor.com \"%s\" not found", name)
}
