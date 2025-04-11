package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
)

var (
	ErrNilUser       = errors.New("user is nil")
	ErrEmptyPassword = errors.New("username is set but password is empty")
	ErrNotLoggedIn   = errors.New("not logged in")
)

func Guest() *Credentials {
	return &Credentials{Username: "guest"}
}

func (user *Credentials) Login() (*Credentials, error) {
	if user == nil {
		return nil, ErrNilUser
	}
	if user.Username != "guest" && user.Password == "" {
		return nil, ErrEmptyPassword
	}
	resp, err := user.PostForm(ApiUrl("login", url.Values{"username": {user.Username}, "password": {user.Password}}), nil)
	user.Password = ""
	if err != nil {
		return nil, fmt.Errorf("error logging in: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var respLog struct {
		Credentials
		RatingsMask string `json:"ratingsmask"`
		UserID      any    `json:"user_id,omitempty"` // temporary override to handle string or number from api
		ErrorResponse
	}
	if err = json.Unmarshal(body, &respLog); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	if respLog.ErrorResponse.Code != nil {
		return nil, fmt.Errorf("error logging in: %s", respLog.ErrorResponse.Message)
	}

	if respLog.Sid == "" {
		return nil, fmt.Errorf("sid is empty, response: %s", body)
	}
	if respLog.UserID != nil {
		switch id := respLog.UserID.(type) {
		case string:
			userID, err := strconv.Atoi(id)
			if err != nil {
				return nil, fmt.Errorf("error parsing userid: %w", err)
			}
			user.UserID = IntString(userID)
		case float64:
			user.UserID = IntString(id)
		case int:
			user.UserID = IntString(id)
		default:
			userID, err := strconv.Atoi(fmt.Sprintf("%v", respLog.UserID))
			if err != nil {
				return nil, fmt.Errorf("error parsing userid: %w", err)
			}
			user.UserID = IntString(userID)
		}
	}

	user.Sid = respLog.Sid
	user.Ratings = parseMask(respLog.RatingsMask)

	return user, nil
}

func (user Credentials) LoggedIn() bool {
	return user.Sid != ""
}

func (user *Credentials) Logout() error {
	if user == nil {
		return ErrNotLoggedIn
	}
	resp, err := user.PostForm(ApiUrl("logout", url.Values{"sid": {user.Sid}}), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := CheckError(body); err != nil {
		return fmt.Errorf("error logging out: %w", err)
	}

	var logoutResp LogoutResponse
	if err = json.Unmarshal(body, &logoutResp); err != nil {
		return err
	}

	if logoutResp.Logout != "success" {
		return fmt.Errorf("logout failed, response: %s", logoutResp.Logout)
	}

	if logoutResp.Sid != user.Sid {
		return fmt.Errorf("session ID changed after logout, expected: [%s], got: [%s]", user.Sid, logoutResp.Sid)
	}

	*user = Credentials{}
	return nil
}
