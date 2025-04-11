package api

import (
	"fmt"
	"strings"
)

type Credentials struct {
	Sid      string    `json:"sid" query:"sid"`
	Username string    `json:"username,omitempty" query:"username"`
	Password string    `json:"password,omitempty" query:"password"`
	UserID   IntString `json:"user_id,omitempty" query:"user_id"`
	Ratings  `json:"ratings,omitempty" query:"ratings"`
}

type Ratings struct {
	// RatingsMask - Binary string representation of the users Allowed Ratings choice. The bits are in this order left-to-right:
	// Eg: A string 11100 means only items rated General, Nudity and Violence are allowed, but Sex and Strong Violence are blocked.
	// A string 11111 means items of any rating would be shown. Only 'left-most significant bits' are returned. So 11010 and 1101 are the same, and 10000 and 1 are the same.
	General        BooleanYN `json:"tag[1],omitempty" query:"tag[1]"` // Show images with Rating tag: General - Suitable for all ages.
	Nudity         BooleanYN `json:"tag[2],omitempty" query:"tag[2]"` // Show images with Rating tag: Nudity - Nonsexual nudity exposing breasts or genitals (must not show arousal).
	MildViolence   BooleanYN `json:"tag[3],omitempty" query:"tag[3]"` // Show images with Rating tag: MildViolence - Mild violence.
	Sexual         BooleanYN `json:"tag[4],omitempty" query:"tag[4]"` // Show images with Rating tag: Sexual Themes - Erotic imagery, sexual activity or arousal.
	StrongViolence BooleanYN `json:"tag[5],omitempty" query:"tag[5]"` // Show images with Rating tag: StrongViolence - Strong violence, blood, serious injury or death.
}

// parseMask returns a Ratings based on a ratings bitmask. True is 1, false is 0
//
//	"11010" would set Ratings{General: true, Nudity: true, MildViolence: false, Sexual: true, StrongViolence: false}
func parseMask(ratingsMask string) Ratings {
	// RatingsMask - Binary string representation of the users Allowed Ratings choice. The bits are in this order left-to-right:
	// Eg: A string 11100 means only items rated General, Nudity and Violence are allowed, but Sex and Strong Violence are blocked.
	// A string 11111 means items of any rating would be shown. Only 'left-most significant bits' are returned. So 11010 and 1101 are the same, and 10000 and 1 are the same.
	set := func(r int32) BooleanYN {
		return r == '1'
	}
	var ratings Ratings
	for i, rating := range ratingsMask {
		switch i {
		case 0:
			ratings.General = set(rating)
		case 1:
			ratings.Nudity = set(rating)
		case 2:
			ratings.MildViolence = set(rating)
		case 3:
			ratings.Sexual = set(rating)
		case 4:
			ratings.StrongViolence = set(rating)
		}
	}
	return ratings
}

// parseBooleans returns a ratings bitmask based on the boolean values of the Ratings struct.
//
//	Ratings{General: true, Nudity: true, MildViolence: false, Sexual: true, StrongViolence: false}
//	would return "1101"
//
// A ratings bitmask is a binary string representation of the users Allowed Ratings choice.
// A string 11100 means only keywords rated General,
// Nudity and Violence are allowed, but Sex and Strong Violence are blocked.
// String 11111 means keywords of any rating would be shown.
// Only 'left-most significant bits' need to be sent.
// So 11010 and 1101 are the same, and 10000 and 1 are the same.
func (ratings Ratings) parseBooleans() string {
	ratingsMask := fmt.Sprintf("%d%d%d%d%d",
		ratings.General.Int(),
		ratings.Nudity.Int(),
		ratings.MildViolence.Int(),
		ratings.Sexual.Int(),
		ratings.StrongViolence.Int(),
	)

	return strings.TrimRight(ratingsMask, "0")
}

func (ratings Ratings) String() string {
	return ratings.parseBooleans()
}

type LogoutResponse struct {
	Sid    string `json:"sid"`
	Logout string `json:"logout"`
}

type WatchlistResponse struct {
	Watches []UsernameID `json:"watches"`
}

type UsernameID struct {
	UserID   string `json:"user_id" query:"user_id"`
	Username string `json:"username" query:"username"`
}

type UsernameAutocomplete struct {
	Results []Autocomplete `json:"results" query:"results"`
}

type Autocomplete struct {
	ID         string `json:"id"`
	Value      string `json:"value"`
	Icon       string `json:"icon"`
	Info       string `json:"info"`
	SingleWord string `json:"singleword"`
	SearchTerm string `json:"searchterm"`
}
