package main

import "fmt"

func main() {
	slice := make([]int, 0, 3)
	fmt.Printf("Initial slice - Ptr: %p\n", slice) // Initial slice - Ptr: 0x...

	slice = append(slice, 1, 2, 3)
	fmt.Printf("Append 1,2,3 - Ptr: %p\n", slice) // Append 1,2,3 - Ptr: 0x...

	slice = append(slice, 4)

    // Append 4 (exceed cap) - Ptr: 0x... // New Pointer!
	fmt.Printf("Append 4 (exceed cap) - Ptr: %p\n", slice)
}
