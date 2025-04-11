package utils

import (
	"fmt"
	"log"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"time"
)

// StructToUrlValues uses reflect to read json struct fields and set them as url.Values
// It also checks if omitempty is set and ignores empty fields
// Example:
//
//	type Example struct {
//		Field1 string `json:"field1,omitempty"`
//		Field2 string `json:"field2"`
//	}
func StructToUrlValues(s any) url.Values {
	return structToUrlValuesRecursive(s, make(url.Values))
}

func structToUrlValuesRecursive(s any, urlValues url.Values) url.Values {
	v := reflect.ValueOf(s)
	// If the passed interface is a pointer, get the value it points to
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()

	// Check if the value is a struct; return immediately if not
	if v.Kind() != reflect.Struct {
		return urlValues
	}

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)
		tag := field.Tag.Get("json")

		if tag == "" {
			continue
		}

		// Handle embedded or nested structs
		if value.Kind() == reflect.Struct && field.Anonymous {
			structToUrlValuesRecursive(value.Interface(), urlValues)
			continue
		} else if value.Kind() == reflect.Struct {
			structToUrlValuesRecursive(value.Interface(), urlValues)
			continue
		}

		// Omit empty fields if "omitempty" is specified
		tagParts := strings.Split(tag, ",")
		if len(tagParts) > 1 && tagParts[1] == "omitempty" && isEmptyValue(value) {
			continue
		}

		if tagParts[0] == "-" {
			continue
		}

		// Use fmt.Stringer interface if implemented
		if stringer, ok := value.Interface().(fmt.Stringer); ok {
			urlValues.Add(tagParts[0], stringer.String())
		} else {
			switch value.Kind() {
			case reflect.Bool:
				if value.Bool() {
					urlValues.Add(tagParts[0], "yes")
				} else {
					urlValues.Add(tagParts[0], "no")
				}
			case reflect.Slice:
				var s []string
				for i := 0; i < value.Len(); i++ {
					s = append(s, fmt.Sprintf("%v", value.Index(i).Interface()))
				}
				urlValues.Add(tagParts[0], strings.Join(s, ","))
			default:
				urlValues.Add(tagParts[0], fmt.Sprintf("%v", value.Interface()))
			}
		}
	}
	return urlValues
}

// Helper function to check if a value is considered empty
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
}

type WatchInfo struct {
	Username  string
	Date      time.Time
	Watching  bool // Watching is true if you are watching the Username
	WatchedBy bool // WatchedBy is true if the Username is watching you
}

func UpdateNewMissing(oldList, newList []string) map[string][]WatchInfo {
	currentTime := time.Now().UTC()
	newWatchlist, newFollows := newFollowers(oldList, newList, currentTime)
	if len(newFollows) > 0 {
		log.Printf("new follows (%d): %v", len(newFollows), newFollows)
	}

	if len(newFollows) == 0 {
		// First run, no previous states
		return newWatchlist
	}

	unfollows := checkMissing(oldList, newWatchlist, currentTime)
	if len(unfollows) > 0 {
		log.Printf("unfollows (%d): %v", len(unfollows), unfollows)
	}

	return newWatchlist
}

// newFollowers checks for new followers by checking if the watchlistStrings doesn't exist in user.Watchers or if the last state is unfollowing.
// If a username is new or previously unfollowed, it's added to the new watchlist with the last state set to following.
// Otherwise, the existing states are copied to the new watchlist.
func newFollowers(oldList, newList []string, currentTime time.Time) (map[string][]WatchInfo, []string) {
	newWatchlist := make(map[string][]WatchInfo)
	var newFollows []string
	for _, username := range newList {
		// check if the username is in the old list
		if !slices.Contains(oldList, username) {
			// username is new
			newWatchlist[username] = append(newWatchlist[username], WatchInfo{Date: currentTime, Watching: true})
			newFollows = append(newFollows, username)
		} else {
			// username is not new
			newWatchlist[username] = append(newWatchlist[username], WatchInfo{Date: currentTime, Watching: false})
		}
	}
	return newWatchlist, newFollows
}

// checkMissing checks for unfollowing by looking at the missing usernames the new watchlist doesn't have that user.Watchers has.
// If a username is missing from the new watchlist, it's added to the new watchlist with the last state set to unfollowing.
func checkMissing(oldList []string, newWatchlist map[string][]WatchInfo, currentTime time.Time) []string {
	var unfollows []string
	for _, username := range oldList {
		states, exists := newWatchlist[username]
		if !exists || !states[len(states)-1].Watching {
			// Last state was user was watching you; now it's not in the new watchers list -> unfollow
			newWatchlist[username] = append(states, WatchInfo{Date: currentTime, Watching: false})
			unfollows = append(unfollows, username)
		}
	}
	return unfollows
}
