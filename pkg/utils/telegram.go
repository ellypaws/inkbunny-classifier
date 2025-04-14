package utils

import (
	"math/rand/v2"

	"gopkg.in/telebot.v4"
)

func CopyButton(button telebot.Btn, data string) telebot.Btn {
	button.Data = data
	return button
}

func Single[button interface{ Inline() *telebot.InlineButton }](buttons ...button) *telebot.ReplyMarkup {
	return NewButtons(buttons)
}

func NewButtons[button interface{ Inline() *telebot.InlineButton }](rows ...[]button) *telebot.ReplyMarkup {
	buttonRows := make([][]telebot.InlineButton, len(rows))
	for i, row := range rows {
		buttonRows[i] = NewRow(row...)
	}

	return &telebot.ReplyMarkup{InlineKeyboard: buttonRows}
}

func NewRow[button interface{ Inline() *telebot.InlineButton }](buttons ...button) []telebot.InlineButton {
	column := make([]telebot.InlineButton, len(buttons))
	for i, b := range buttons {
		column[i] = *b.Inline()
	}

	return column
}

func Random[T any](v ...T) T {
	if len(v) == 0 {
		var def T
		return def
	}
	return v[rand.IntN(len(v))]
}

func RandomActivity() telebot.ChatAction {
	return Random(
		telebot.Typing,
		telebot.UploadingPhoto,
		telebot.UploadingVideo,
		telebot.UploadingAudio,
		telebot.UploadingDocument,
		telebot.UploadingVNote,
		telebot.RecordingVideo,
		telebot.RecordingAudio,
		telebot.RecordingVNote,
		telebot.FindingLocation,
		telebot.ChoosingSticker,
	)
}
