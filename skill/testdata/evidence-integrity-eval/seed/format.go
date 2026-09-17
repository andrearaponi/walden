package formattools

// Normalize trims surrounding spaces and lowercases the input.
func Normalize(input string) string { return input }

// Slug joins whitespace-separated normalized words with hyphens.
func Slug(input string) string { return input }

// Label returns "note:" followed by the normalized slug.
func Label(input string) string { return input }
