module classifier

go 1.24.2

replace github.com/ellypaws/inkbunny/api => ./cmd/dataset/vendor/github.com/ellypaws/inkbunny/api

require (
	github.com/charmbracelet/log v0.4.1
	github.com/ellypaws/inkbunny/api v0.0.0-20240523184311-b8d31bbdc865
	github.com/joho/godotenv v1.5.1
	github.com/lucasb-eyer/go-colorful v1.2.0
	github.com/muesli/termenv v0.16.0
	golang.org/x/image v0.26.0
	gopkg.in/telebot.v4 v4.0.0-beta.4
)

require (
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/charmbracelet/lipgloss v1.0.0 // indirect
	github.com/charmbracelet/x/ansi v0.4.2 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d // indirect
	golang.org/x/sys v0.30.0 // indirect
)
