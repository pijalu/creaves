package utils

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gobuffalo/nulls"
)

// TrimStringFields trims spaces at the end of all string and nulls.String fields in the given struct
func TrimStringFields(s interface{}) {
	// Use reflection to inspect the struct
	v := reflect.ValueOf(s)

	// Ensure the input is a pointer to a struct
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		fmt.Println("Input must be a pointer to a struct")
		return
	}

	// Get the actual struct value
	v = v.Elem()

	// Iterate through the struct fields
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)

		// Check if the field is a string and can be modified
		if field.Kind() == reflect.String && field.CanSet() {
			trimmed := strings.TrimRight(field.String(), " ")
			field.SetString(trimmed)
		}

		// Check for nulls.String type
		if field.Type() == reflect.TypeOf(nulls.String{}) && field.CanSet() {
			// Get the current nulls.String value
			ns := field.Interface().(nulls.String)
			if ns.Valid { // Only process if Valid is true
				ns.String = strings.TrimRight(ns.String, " ")
				field.Set(reflect.ValueOf(ns)) // Set the modified value back
			}
		}
	}
}
