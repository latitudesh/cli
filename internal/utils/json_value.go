package utils

import (
	"reflect"
	"strings"
)

func AssignValue(obj interface{}, jsonName string, value interface{}) {
	fieldValue := GetFieldValue(obj, jsonName)

	if fieldValue.IsValid() && fieldValue.CanSet() {
		valueToSet := reflect.ValueOf(value)

		// Prepare for user passing project Name instead of Slug.
		if jsonName == "project" {
			parametrizedProject := strings.ToLower(strings.Replace(valueToSet.String(), " ", "-", -1))
			valueToSet = reflect.ValueOf(parametrizedProject)
		}

		fieldValue.Set(valueToSet)
	}
}

func GetFieldValue(obj interface{}, jsonName string) reflect.Value {
	objValue := reflect.ValueOf(obj).Elem()

	for i := 0; i < objValue.NumField(); i++ {
		field := objValue.Type().Field(i)
		jsonFieldName := readJSONFieldName(field, objValue)

		if jsonFieldName == jsonName {
			return objValue.FieldByName(field.Name)
		}
	}

	return objValue
}

func readJSONFieldName(field reflect.StructField, value reflect.Value) string {
	tag := field.Tag.Get("json")
	return strings.Split(tag, ",")[0]
}
