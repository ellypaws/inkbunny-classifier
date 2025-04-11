<p align="center">
  <img src="https://inkbunny.net/images81/elephant/logo/bunny.png" width="100" />
  <img src="https://inkbunny.net/images81/elephant/logo/text.png" width="300" />
  <br>
  <h1 align="center">Inkbunny API</h1>
</p>

<p align="center">
  <a href="https://inkbunny.net/">
    <img alt="Inkbunny" src="https://img.shields.io/badge/website-inkbunny.net-blue">
  </a>
  <a href="https://wiki.inkbunny.net/wiki/API">
    <img alt="API" src="https://img.shields.io/badge/api-wiki.inkbunny.net-blue">
  </a>
  <a href="https://pkg.go.dev/github.com/ellypaws/inkbunny/api">
    <img alt="go.dev reference" src="https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white">
  </a>
  <a href="https://goreportcard.com/report/github.com/ellypaws/inkbunny/api">
    <img src="https://goreportcard.com/badge/github.com/ellypaws/inkbunny/api" alt="Go Report Card" />
  </a>
  <br>
  <a href="https://github.com/ellypaws/inkbunny/graphs/contributors">
    <img alt="Inkbunny API contributors" src="https://img.shields.io/github/contributors/ellypaws/inkbunny">
  </a>
  <a href="https://github.com/ellypaws/inkbunny/commits/main">
    <img alt="Commit Activity" src="https://img.shields.io/github/commit-activity/m/ellypaws/inkbunny">
  </a>
  <a href="https://github.com/ellypaws/inkbunny">
    <img alt="GitHub Repo stars" src="https://img.shields.io/github/stars/ellypaws/inkbunny?style=social">
  </a>
</p>

--------------

<p align="right"><i>Disclaimer: This project is not affiliated or endorsed by Inkbunny.</i></p>

Inkbunny API is a Go package that provides a simple way to interact with the Inkbunny API. It allows you to log in, log
out, and make requests to the Inkbunny API.

It aims to provide all of the available API endpoints and methods to interact with the platform.
The necessary structs and methods are abstracted away so that you can send and receive data in standardized Go structs.

Table of Contents
=================

- [Table of Contents](#table-of-contents)
    - [Installation](#installation)
    - [API Methods](#api-methods)
    - [Usage](#usage)
    - [Learn More](#learn-more)
    - [Contributing](#contributing)

## Installation

<img src="https://go.dev/images/gophers/ladder.svg" width="48" alt="Go Gopher climbing a ladder." align="right">

To use the api module, you need to have Go installed on your system. If you don't have Go installed, you can
download it from the [official Go website](https://golang.org/dl/).

Once you have Go installed, you can get the package by running the following command:

```bash
go get github.com/ellypaws/inkbunny/api
```

## Usage

Here's a simple example of how to use the Inkbunny API package logging in as
a [guest](https://wiki.inkbunny.net/wiki/API#Quick_Start_Guide):

```go
package main

import (
  "github.com/ellypaws/inkbunny/api"
  "log"
)

func main() {
  user, err := api.Guest().Login()
  if err != nil {
    log.Printf("Error logging in: %v", err)
    return
  }

  log.Printf("Logged in with session ID: %s", user.Sid)
}

```

You can login with your own credentials by creating a `Credentials` object and calling the `Login` method.
Note that the password gets destroyed after the login request is made.
Important: Do not hardcode your credentials in your code.
You can use environment variables or term `"golang.org/x/term"` to input your credentials.

```go
package main

import (
  "fmt"
  "github.com/ellypaws/inkbunny/api"
  "log"
)

func main() {
  user := &api.Credentials{
    Username: "your_username",
    Password: "your_password",
  }

  user, err := user.Login()
  if err != nil {
    log.Printf("Error logging in: %v", err)
    return
  }

  log.Printf("Logged in with session ID: %s", user.Sid)

  if err := user.Logout(); err != nil {
    log.Printf("Error logging out: %v", err)
    return
  }

  fmt.Println("Logged out")
}
```

Because the API uses "yes" or "no" to represent boolean values, use `api.Yes` and `api.No` to represent these values.

```go
package main

import (
  "github.com/ellypaws/inkbunny/api"
  "log"
)

func main() {
  user, err := api.Guest().Login()
  if err != nil {
    log.Fatalf("Error logging in: %v", err)
  }

  if err := user.ChangeRating(api.Ratings{
    General:      api.Yes,
    Nudity:       api.No,
    MildViolence: api.Yes,
  }); err != nil {
    log.Fatalf("Error changing rating: %v", err)
  }

  user.SubmissionDetails(
    api.SubmissionDetailsRequest{
      SubmissionIDs:   "your submission IDs here",
      ShowDescription: api.Yes,
    },
  )
}
```

## API Methods

The application provides several API methods to interact with the platform:
(note: We always need a `Credentials` object to call these methods)

| Method                                                                                                  | Description                                               |
|---------------------------------------------------------------------------------------------------------|-----------------------------------------------------------| 
| `(user *Credentials) Login() (*Credentials, error)`                                                     | Logs in a user.                                           |
| `(user *Credentials) Logout() error`                                                                    | Logs out a user.                                          |
| `(user Credentials) LoggedIn() bool`                                                                    | Checks if a user is logged in.                            |
| `(user Credentials) Request(method string, url string, body io.Reader) (*http.Request, error)`          | Sends a request to the specified URL.                     |
| `(user Credentials) Get(url *url.URL) (*http.Response, error)`                                          | Sends a GET request to the specified URL.                 |
| `(user Credentials) Post(url *url.URL, contentType string, body io.Reader) (*http.Response, error)`     | Sends a POST request to the specified URL.                |
| `(user Credentials) PostForm(url *url.URL, values url.Values) (*http.Response, error)`                  | Sends a POST request with form data to the specified URL. |
| `(user Credentials) GetWatching() ([]UsernameID, error)`                                                | Retrieves the user's watchlist. (users you're watching)   |
| `(user Credentials) SubmissionDetails(req SubmissionDetailsRequest) (SubmissionDetailsResponse, error)` | Retrieves the details of a submission.                    |
| `(user Credentials) SubmissionFavorites(req SubmissionRequest) (SubmissionFavoritesResponse, error)`    | Retrieves the favorites of a submission.                  |
| `(user Credentials) SearchSubmissions(req SearchRequest) (SearchResponse, error)`                       | Searches for submissions.                                 |
| `(user Credentials) OwnSubmissions() (SearchResponse, error)`                                           | Retrieves the user's own submissions.                     |
| `(user Credentials) UserSubmissions(username string) (SearchResponse, error)`                           | Retrieves the submissions of a specified user.            |

## Learn More

You can learn more about the Inkbunny API by visiting
the [official Inkbunny API wiki](https://wiki.inkbunny.net/wiki/API).

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.