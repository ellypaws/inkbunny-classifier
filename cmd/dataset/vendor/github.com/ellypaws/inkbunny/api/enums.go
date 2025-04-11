package api

import (
	"encoding/json"
	"fmt"
	"iter"
	"strconv"
	"strings"
)

// OutputMode is a custom type to handle the output mode of the API response. Valid values are ["json","xml"]. Defaults to "json".
type OutputMode string

const (
	JSON OutputMode = "json"
	XML  OutputMode = "xml" // Deprecated: Do not use it with JSON parsing, it will most likely error.
)

// MarshalJSON converts OutputMode enum into ["json","xml"]. Defaults to "json".
func (o OutputMode) MarshalJSON() ([]byte, error) {
	if o == "" {
		return json.Marshal("json")
	}
	return json.Marshal(o)
}

// UnmarshalJSON parses the JSON string into an OutputMode type.
func (o *OutputMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "json":
		*o = JSON
	case "xml": // Technically, UnmarshalJSON is not used for XML, but it's here for completeness. Will most likely error if used with JSON parsing.
		*o = XML
	default:
		return fmt.Errorf(`allowed values for OutputMode ["json","xml"], got "%s"`, s)
	}
	return nil
}

// BooleanYN is a custom type to handle boolean values marshaled as "yes" or "no".
type BooleanYN bool

const (
	Yes BooleanYN = true
	No  BooleanYN = false

	True  BooleanYN = true
	False BooleanYN = false
)

// MarshalJSON converts the BooleanYN boolean into a JSON string of "yes" or "no".
// Typically used for requests as part of url.Values.
func (b BooleanYN) MarshalJSON() ([]byte, error) {
	if b {
		return json.Marshal("yes")
	}
	return json.Marshal("no")
}

// UnmarshalJSON parses string booleans into a BooleanYN type.
// Typically, responses returns "t" or "f" for true and false, while requests use "yes" and "no".
func (b *BooleanYN) UnmarshalJSON(data []byte) error {
	var d any
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	switch d := d.(type) {
	case string:
		switch d {
		case "t", "yes", "true":
			*b = true
		case "f", "no", "false":
			*b = false
		default:
			return fmt.Errorf(`allowed values for Boolean [t, f], [yes, no], [true, false], got %s`, d)
		}
	case bool:
		*b = BooleanYN(d)
	default:
		return fmt.Errorf("invalid type for boolean: %T", d)
	}
	return nil
}

func (b BooleanYN) String() string {
	if b {
		return "yes"
	}
	return "no"
}

func (b BooleanYN) Int() int {
	if b {
		return 1
	}
	return 0
}

func (b BooleanYN) Bool() bool {
	return bool(b)
}

// IntString is a custom type to handle int values marshaled as strings. Typically only returned by responses.
type IntString int

func (i IntString) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

func (i *IntString) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if string(data) == "null" {
		return nil
	}
	atoi, err := strconv.Atoi(strings.ReplaceAll(string(data), `"`, ""))
	if err != nil {
		return fmt.Errorf("failed to convert data %s to int: %w", data, err)
	}
	*i = IntString(atoi)
	return nil
}

func (i IntString) String() string {
	return strconv.Itoa(int(i))
}

func (i IntString) Int() int {
	return int(i)
}

func (i IntString) Iter() iter.Seq[IntString] {
	return func(yield func(IntString) bool) {
		for i := range i.Int() {
			if !yield(IntString(i)) {
				return
			}
		}
	}
}

// PriceString is a custom type to handle float64 values marshaled as strings ($USD). Typically only returned by responses.
type PriceString float64

func (i PriceString) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.Itoa(int(i)))
}

func (i *PriceString) UnmarshalJSON(data []byte) error {
	_, err := fmt.Sscanf(strings.ReplaceAll(string(data), `"`, ""), `$%f`, i)
	if err != nil {
		return fmt.Errorf("failed to convert data %s to float64: %w", data, err)
	}
	return nil
}

func (i PriceString) String() string {
	return fmt.Sprintf("$%.2f", i)
}

func (i PriceString) Float() float64 {
	return float64(i)
}

type JoinType string

const (
	And JoinType = "and"
	Or  JoinType = "or"
)
