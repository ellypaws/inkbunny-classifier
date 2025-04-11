package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"
)

type WatchInfo struct {
	Username  string
	Date      time.Time
	Watching  bool // Watching is true if you are watching the Username
	WatchedBy bool // WatchedBy is true if the Username is watching you
}

// GetWatching gets the watchlist of a logged-in user
func (user Credentials) GetWatching() ([]UsernameID, error) {
	resp, err := user.PostForm(ApiUrl("watchlist", url.Values{"sid": {user.Sid}}), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := CheckError(body); err != nil {
		return nil, fmt.Errorf("error getting watchlist: %w", err)
	}

	var watchResp WatchlistResponse
	if err := json.Unmarshal(body, &watchResp); err != nil {
		return nil, err
	}

	return watchResp.Watches, nil
}
