package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ellypaws/inkbunny/api/utils"
)

// SubmissionDetailsRequest is modified to use BooleanYN for fields requiring "yes" or "no" representation.
type SubmissionDetailsRequest struct {
	SID                         string     `json:"sid" query:"sid"`
	SubmissionIDs               string     `json:"submission_ids" query:"submission_ids"` // SubmissionIDs is a comma-separated list of submission IDs
	SubmissionIDSlice           []string   // SubmissionIDSlice will be joined as a comma-separated into SubmissionIDs
	OutputMode                  OutputMode `json:"output_mode" query:"output_mode"`
	SortKeywordsBy              string     `json:"sort_keywords_by" query:"sort_keywords_by"`
	ShowDescription             BooleanYN  `json:"show_description" query:"show_description"`
	ShowDescriptionBbcodeParsed BooleanYN  `json:"show_description_bbcode_parsed" query:"show_description_bbcode_parsed"`
	ShowWriting                 BooleanYN  `json:"show_writing" query:"show_writing"`
	ShowWritingBbcodeParsed     BooleanYN  `json:"show_writing_bbcode_parsed" query:"show_writing_bbcode_parsed"`
	ShowPools                   BooleanYN  `json:"show_pools" query:"show_pools"`
}

type SubmissionBasic struct {
	SubmissionID     string    `json:"submission_id"`
	Hidden           BooleanYN `json:"hidden"`
	Username         string    `json:"username"`
	UserID           string    `json:"user_id"`
	CreateDateSystem string    `json:"create_datetime"`
	CreateDateUser   string    `json:"create_datetime_usertime"`
	UpdateDateSystem string    `json:"last_file_update_datetime,omitempty"`
	UpdateDateUser   string    `json:"last_file_update_datetime_usertime,omitempty"`
	FileName         string    `json:"file_name"`
	LatestFileName   string    `json:"latest_file_name"`
	Title            string    `json:"title"`
	Deleted          BooleanYN `json:"deleted"`
	Public           BooleanYN `json:"public"`
	MimeType         string    `json:"mimetype"`
	LatestMimeType   string    `json:"latest_mimetype"`
	PageCount        IntString `json:"pagecount"`
	RatingID         IntString `json:"rating_id"`
	RatingName       string    `json:"rating_name"`
	FileURL
	Thumbs
	LatestThumbs
	SubmissionTypeID IntString `json:"submission_type_id"`
	TypeName         string    `json:"type_name"`
	Digitalsales     BooleanYN `json:"digitalsales"`
	Printsales       BooleanYN `json:"printsales"`
	FriendsOnly      BooleanYN `json:"friends_only"`
	GuestBlock       BooleanYN `json:"guest_block"`
	Scraps           BooleanYN `json:"scraps"`
}

type UserIconURLs struct {
	Large  string `json:"user_icon_url_large,omitempty"`
	Medium string `json:"user_icon_url_medium,omitempty"`
	Small  string `json:"user_icon_url_small,omitempty"`
}

type Submission struct {
	SubmissionBasic
	Keywords         []Keyword `json:"keywords"`
	Favorite         BooleanYN `json:"favorite"`
	FavoritesCount   IntString `json:"favorites_count"`
	UserIconFileName string    `json:"user_icon_file_name"`
	UserIconURLs
	LatestFileURL
	Files                   []File             `json:"files"`
	Pools                   []Pool             `json:"pools"`
	Description             string             `json:"description"`
	DescriptionBBCodeParsed string             `json:"description_bbcode_parsed"`
	Writing                 string             `json:"writing"`
	WritingBBCodeParsed     string             `json:"writing_bbcode_parsed"`
	PoolsCount              int                `json:"pools_count"`
	Ratings                 []SubmissionRating `json:"ratings"`
	CommentsCount           IntString          `json:"comments_count"`
	Views                   IntString          `json:"views"`
	SalesDescription        string             `json:"sales_description"`
	ForSale                 BooleanYN          `json:"forsale"`
	DigitalPrice            IntString          `json:"digital_price"`
	Prints                  []Print            `json:"prints"`
}

type Keyword struct {
	KeywordID   string    `json:"keyword_id"`
	KeywordName string    `json:"keyword_name"`
	Suggested   BooleanYN `json:"contributed"`
	Count       IntString `json:"submissions_count"`
}

type File struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	Thumbs
	FileURL
	MimeType            string    `json:"mimetype"`
	SubmissionID        string    `json:"submission_id"`
	UserID              string    `json:"user_id"`
	SubmissionFileOrder IntString `json:"submission_file_order"`
	FullSizeX           IntString `json:"full_size_x"`
	FullSizeY           IntString `json:"full_size_y"`
	ThumbHugeX          IntString `json:"thumb_huge_x,omitempty"`
	ThumbHugeY          IntString `json:"thumb_huge_y,omitempty"`
	ThumbNonCustomX     IntString `json:"thumb_huge_noncustom_x,omitempty"`
	ThumbNonCustomY     IntString `json:"thumb_huge_noncustom_y,omitempty"`
	InitialFileMD5      string    `json:"initial_file_md5"`
	FullFileMD5         string    `json:"full_file_md5"`
	LargeFileMD5        string    `json:"large_file_md5"`
	SmallFileMD5        string    `json:"small_file_md5"`
	ThumbnailMD5        string    `json:"thumbnail_md5"`
	Deleted             BooleanYN `json:"deleted"`
	CreateDateTime      string    `json:"create_datetime"`
	CreateDateTimeUser  string    `json:"create_datetime_usertime"`
}

// FileURL is the Full URL of the (SIZE) asset for the PRIMARY file of this submission. SIZE can be one of "full, screen, preview".
type FileURL struct {
	FileURLFull    string `json:"file_url_full,omitempty"`
	FileURLScreen  string `json:"file_url_screen,omitempty"`
	FileURLPreview string `json:"file_url_preview,omitempty"`
}

type Pool struct {
	PoolID                     string    `json:"pool_id"`
	Name                       string    `json:"name"`
	Description                string    `json:"description"`
	Count                      IntString `json:"count"`
	LeftSubmissionID           string    `json:"submission_left_submission_id"`
	RightSubmissionID          string    `json:"submission_right_submission_id"`
	LeftSubmissionFileName     string    `json:"submission_left_file_name"`
	RightSubmissionFileName    string    `json:"submission_right_file_name"`
	LeftThumbnailURL           string    `json:"submission_left_thumbnail_url,omitempty"`
	RightThumbnailURL          string    `json:"submission_right_thumbnail_url,omitempty"`
	LeftThumbnailURLNonCustom  string    `json:"submission_left_thumbnail_url_noncustom,omitempty"`
	RightThumbnailURLNonCustom string    `json:"submission_right_thumbnail_url_noncustom,omitempty"`
	LeftThumbX                 IntString `json:"submission_left_thumb_huge_x,omitempty"`
	LeftThumbY                 IntString `json:"submission_left_thumb_huge_y,omitempty"`
	RightThumbX                IntString `json:"submission_right_thumb_huge_x,omitempty"`
	RightThumbY                IntString `json:"submission_right_thumb_huge_y,omitempty"`
	LeftThumbNonCustomX        IntString `json:"submission_left_thumb_huge_noncustom_x,omitempty"`
	LeftThumbNonCustomY        IntString `json:"submission_left_thumb_huge_noncustom_y,omitempty"`
	RightThumbNonCustomX       IntString `json:"submission_right_thumb_huge_noncustom_x,omitempty"`
	RightThumbNonCustomY       IntString `json:"submission_right_thumb_huge_noncustom_y,omitempty"`
}

type Print struct {
	PrintSizeID        IntString   `json:"print_size_id"`
	Name               string      `json:"name"`
	Price              PriceString `json:"price"`
	PriceOwnerDiscount PriceString `json:"price_owner_discount,omitempty"`
}
type SubmissionRating struct {
	ContentTagID IntString `json:"content_tag_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	RatingID     IntString `json:"rating_id"`
}

// LatestFileURL Full URL of the (SIZE) asset for the LATEST added file of this submission. SIZE can be one of "full, screen, preview".
type LatestFileURL struct {
	LatestFileURLFull    string `json:"latest_file_url_full"`
	LatestFileURLScreen  string `json:"latest_file_url_screen"`
	LatestFileURLPreview string `json:"latest_file_url_preview"`
}

type LatestThumbs struct {
	LatestThumbnailURLMedium          string    `json:"latest_thumbnail_url_medium,omitempty"`
	LatestThumbnailURLMediumNonCustom string    `json:"latest_thumbnail_url_medium_noncustom,omitempty"`
	LatestThumbnailURLLarge           string    `json:"latest_thumbnail_url_large,omitempty"`
	LatestThumbnailURLLargeNonCustom  string    `json:"latest_thumbnail_url_large_noncustom,omitempty"`
	LatestThumbnailURLHuge            string    `json:"latest_thumbnail_url_huge,omitempty"`
	LatestThumbnailURLHugeNonCustom   string    `json:"latest_thumbnail_url_huge_noncustom,omitempty"`
	LatestThumbMediumX                IntString `json:"latest_thumb_medium_x,omitempty"`
	LatestThumbLargeX                 IntString `json:"latest_thumb_large_x,omitempty"`
	LatestThumbHugeX                  IntString `json:"latest_thumb_huge_x,omitempty"`
	LatestThumbMediumY                IntString `json:"latest_thumb_medium_y,omitempty"`
	LatestThumbLargeY                 IntString `json:"latest_thumb_large_y,omitempty"`
	LatestThumbHugeY                  IntString `json:"latest_thumb_huge_y,omitempty"`
	LatestThumbMediumNonCustomX       IntString `json:"latest_thumb_medium_noncustom_x,omitempty"`
	LatestThumbLargeNonCustomX        IntString `json:"latest_thumb_large_noncustom_x,omitempty"`
	LatestThumbHugeNonCustomX         IntString `json:"latest_thumb_huge_noncustom_x,omitempty"`
	LatestThumbMediumNonCustomY       IntString `json:"latest_thumb_medium_noncustom_y,omitempty"`
	LatestThumbLargeNonCustomY        IntString `json:"latest_thumb_large_noncustom_y,omitempty"`
	LatestThumbHugeNonCustomY         IntString `json:"latest_thumb_huge_noncustom_y,omitempty"`
}

type Thumbs struct {
	ThumbnailURLMedium          string `json:"thumbnail_url_medium,omitempty"`
	ThumbnailURLMediumNonCustom string `json:"thumbnail_url_medium_noncustom,omitempty"`
	ThumbnailURLLarge           string `json:"thumbnail_url_large,omitempty"`
	ThumbnailURLLargeNonCustom  string `json:"thumbnail_url_large_noncustom,omitempty"`
	ThumbnailURLHuge            string `json:"thumbnail_url_huge,omitempty"`
	ThumbnailURLHugeNonCustom   string `json:"thumbnail_url_huge_noncustom,omitempty"`
	ThumbnailDimensions
}

type ThumbnailDimensions struct {
	ThumbMediumX          IntString `json:"thumb_medium_x,omitempty"`
	ThumbLargeX           IntString `json:"thumb_large_x,omitempty"`
	ThumbHugeX            IntString `json:"thumb_huge_x,omitempty"`
	ThumbMediumY          IntString `json:"thumb_medium_y,omitempty"`
	ThumbLargeY           IntString `json:"thumb_large_y,omitempty"`
	ThumbHugeY            IntString `json:"thumb_huge_y,omitempty"`
	ThumbMediumNonCustomX IntString `json:"thumb_medium_noncustom_x,omitempty"`
	ThumbLargeNonCustomX  IntString `json:"thumb_large_noncustom_x,omitempty"`
	ThumbHugeNonCustomX   IntString `json:"thumb_huge_noncustom_x,omitempty"`
	ThumbMediumNonCustomY IntString `json:"thumb_medium_noncustom_y,omitempty"`
	ThumbLargeNonCustomY  IntString `json:"thumb_large_noncustom_y,omitempty"`
	ThumbHugeNonCustomY   IntString `json:"thumb_huge_noncustom_y,omitempty"`
}

type SubmissionDetailsResponse struct {
	Sid          string       `json:"sid"`
	ResultsCount int          `json:"results_count"`
	UserLocation string       `json:"user_location"`
	Submissions  []Submission `json:"submissions"`
}

func (user Credentials) SubmissionDetails(req SubmissionDetailsRequest) (SubmissionDetailsResponse, error) {
	if !user.LoggedIn() {
		return SubmissionDetailsResponse{}, ErrNotLoggedIn
	}
	if req.SID == "" {
		req.SID = user.Sid
	}

	if len(req.SubmissionIDSlice) > 0 {
		if req.SubmissionIDs != "" {
			req.SubmissionIDs += ","
		}
		req.SubmissionIDs += strings.Join(req.SubmissionIDSlice, ",")
	}

	urlValues := utils.StructToUrlValues(req)

	if !urlValues.Has("submission_ids") && len(req.SubmissionIDSlice) > 0 {
		urlValues.Set("submission_ids", strings.Join(req.SubmissionIDSlice, ","))
	}

	resp, err := user.PostForm(ApiUrl("submissions", urlValues), nil)
	if err != nil {
		return SubmissionDetailsResponse{}, fmt.Errorf("failed to get submission details: %w", err)
	}
	defer resp.Body.Close()

	bin, err := io.ReadAll(resp.Body)
	if err != nil {
		return SubmissionDetailsResponse{}, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := CheckError(bin); err != nil {
		return SubmissionDetailsResponse{}, fmt.Errorf("error getting submission details: %w", err)
	}

	var submission SubmissionDetailsResponse
	if err := json.Unmarshal(bin, &submission); err != nil {
		return SubmissionDetailsResponse{}, fmt.Errorf("failed to unmarshal submission details: %w", err)
	}
	return submission, nil
}

type SubmissionRequest struct {
	SID          string     `json:"sid" query:"sid"`
	SubmissionID string     `json:"submission_id" query:"submission_id"`
	OutputMode   OutputMode `json:"output_mode,omitempty" query:"output_mode"`
}

type SubmissionFavoritesResponse struct {
	Sid   string       `json:"sid"`
	Users []UsernameID `json:"favingusers"`
}

func (user Credentials) SubmissionFavorites(req SubmissionRequest) (SubmissionFavoritesResponse, error) {
	if !user.LoggedIn() {
		return SubmissionFavoritesResponse{}, ErrNotLoggedIn
	}
	if req.SID == "" {
		req.SID = user.Sid
	}

	bin, err := json.Marshal(req)
	if err != nil {
		return SubmissionFavoritesResponse{}, err
	}

	resp, err := user.Post(ApiUrl("submissionfavingusers"), MimeTypeJSON, bytes.NewReader(bin))
	if err != nil {
		return SubmissionFavoritesResponse{}, err
	}
	defer resp.Body.Close()

	bin, err = io.ReadAll(resp.Body)
	if err != nil {
		return SubmissionFavoritesResponse{}, err
	}

	if err := CheckError(bin); err != nil {
		return SubmissionFavoritesResponse{}, fmt.Errorf("error getting favorites: %w", err)
	}

	var favorites SubmissionFavoritesResponse
	if err := json.NewDecoder(resp.Body).Decode(&favorites); err != nil {
		return SubmissionFavoritesResponse{}, err
	}
	return favorites, nil
}
