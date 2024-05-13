package main

import "fmt"

func main() {
	fmt.Printf("Testing stuff\n")

	args := []interface{}{}

	args = append(args, "Hello")
	args = append(args, "World")

	fmt.Printf("%s %s\n", args...)
}
