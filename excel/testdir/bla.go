package main

import (
	"fmt"
)

func convertToExcelNotation(line, col int) string {
	result := ""

	for col > 0 {
		mod := (col - 1) % 26
		result = string('A'+mod) + result
		col = (col - 1) / 26
	}

	return fmt.Sprintf("%s%d", result, line)
}

func main() {
	line, col := 1, 56
	excelNotation := convertToExcelNotation(line, col)
	fmt.Printf("Line %d Col %d in Excel notation: %s\n", line, col, excelNotation)

	line, col = 132, 3
	excelNotation = convertToExcelNotation(line, col)
	fmt.Printf("Line %d Col %d in Excel notation: %s\n", line, col, excelNotation)
}
