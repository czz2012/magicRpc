package common

import "strings"

//MethodSplit Return class.method
func MethodSplit(name string) []string {
	return strings.Split(name, ".")
}
