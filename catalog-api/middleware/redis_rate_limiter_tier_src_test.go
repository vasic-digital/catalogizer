package middleware

import "os"

func mainGoSource() (string, error) {
	b, err := os.ReadFile("../main.go")
	return string(b), err
}
