# Go Cerb!

Implements Webgroup Media's [Cerb Helpdesk API](https://cerb.ai/docs/api/) in Golang.

## Setup 

Ensure you [enable the API plugin](https://cerb.ai/guides/api/configure-plugin/) on your Cerb instance and generate an API key-pair. Be sure to set the endpoints that you want to allow.

## Usage

[Install Go](https://golang.org/doc/install) and make sure your [GOPATH](https://golang.org/doc/code.html#GOPATH) is set.

`go get github.com/dteare/gocerb`

Copy `sample-creds.json` to `~/.config/cerb/creds.json` and update it with your API key-pair that you created.

## Testing

Update `cerb.NewCerberus` with your base server URL in `main.go`. You'll also need to set your Bucket and Group ids in `testCreateTicket`.

Run `go run main.go` and you should see:

```
$ go run main.go
Create message 3983875 within ticket 3983875! 💌 https://agilebits.cerb.me/profiles/ticket/IHQ-29388-848
Found 7 open tickets for dave+gocerb@1password.com.
Loaded 100 tickets from page 0. 2312 tickets remain on subsequent pages.
Loaded 100 tickets from page 1. 2212 tickets remain on subsequent pages.
...
Loaded 100 tickets from page 23. 11 tickets remain on subsequent pages.
Loaded 100 tickets from page 24. 0 tickets remain on subsequent pages.
```

## Contributing

GoCerb! was primarily created to help our customer support team at [1Password](https://1password.com). I'll happily review pull requests and merge those that help us help more customers. My email notifications are out of control so [ping me on Twitter](https://twitter.com/dteare) to get my attention. 

## Contributors

A big hat tip to [@jstanden](https://github.com/jstanden) for all his help making this possible. 😘
