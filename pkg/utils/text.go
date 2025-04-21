package utils

func Plural(i int) string {
	if i > 1 {
		return "s"
	}
	return ""
}
