package main

import (
	"encoding/json"
	"fmt"
)

// Formatter interface defines a method for outputting messages
type Formatter interface {
	Output(message string) string
}

// OutputFunc is a function type that matches the signature of the Output
// method in the Formatter interface
type OutputFunc func(message string) string

// Output method makes OutputFunc satisfy the Formatter interface
func (f OutputFunc) Output(message string) string {
	return f(message)
}

func Display(message string, format Formatter) {
	fmt.Println(format.Output(message))
}

func main() {
	message := "Hello, World!"

	TextFormatted := OutputFunc(func (message string) string {
		return message
	})

	JSONFormatted := OutputFunc(func (message string) string {
		jsonData, _ := json.Marshal(map[string]string{"message": message})
		return string(jsonData)
	})

	Display(message, TextFormatted)
	Display(message, JSONFormatted)
}
