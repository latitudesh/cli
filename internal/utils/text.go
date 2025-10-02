package utils

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func TitleStr(str string) string {
	caser := cases.Title(language.English)
	return caser.String(str)
}

func Singular(str string) string {
	return strings.TrimSuffix(str, "s")
}
