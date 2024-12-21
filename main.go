package main

import (
	"bytes"
	"fmt"
)

// LoggingWriter is an io.Writer that logs stats before writing to an underlying writer.
type LoggingWriter struct {}

// Write logs the number of bytes being written and delegates the write to the underlying writer.
func (lw *LoggingWriter) Write(data []byte) (int, error) {
    // Log stats
    fmt.Printf("LoggingWriter: Writing %d bytes\n", len(data))

    // Write to the underlying writer
    return lw.w.Write(data)
}

func main() {
	// Create a buffer as the underlying writer
	var buf bytes.Buffer

	// Define a WriteFunc that logs stats before writing to the buffer
	logWriter := WriteFunc(func(data []byte) (int, error) {
		// Log stats
		fmt.Printf("WriteFunc: Writing %d bytes\n", len(data))

		// Write to the buffer
		return buf.Write(data)
	})

	// Write some data using the logWriter
	_, err := logWriter.Write([]byte("Hello, world!"))
	if err != nil {
		fmt.Println("Error writing data:", err)
		return
	}

	// Print the content of the buffer
	fmt.Println("Buffer content:", buf.String())
}
