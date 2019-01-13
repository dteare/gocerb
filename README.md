
# Go Cerb!

Implements Webgroup Media's [Cerb Helpdesk API](https://cerb.ai/docs/api/).

## Setup 

Ensure you [enable the API plugin](https://cerb.ai/guides/api/configure-plugin/) on your Cerb instance and generate an API key-pair. Be sure to set the endpoints that you want to allow.

## Usage

[Install Go](https://golang.org/doc/install) and make sure your [GOPATH](https://golang.org/doc/code.html#GOPATH) is set.

`go get github.com/dteare/gocerb`

Copy `sample-creds.json` to `~/.config/cerb/creds.json` and update it with your API key-pair that you created.

## Testing

Run `go run main.go` and you should see:

```
$ go run main.go
Create message 3968929 within ticket 1264037.
Created message! 💌 https://agilebits.cerb.me/profiles/ticket/JQW-58124-144
req.URL= https://agilebits.cerb.me/rest/records/ticket/search.json?q=status%3A%5Bo%5D+messages.first%3A%28sender%3A%28email%3A%27dave%2Bgocerb%401password.com%27%29%29
Found 0 open tickets for dave+gocerb@1password.com.
Loaded 100 tickets from page 0. 3120 tickets remain on subsequent pages.
Page 0 has 100 tickets. Has more? true
```

Note the sample app looks for open tickets in the Billing and Sales groups only.


## Contributors

A big hat tip to [@jstanden](https://github.com/jstanden) for all his help making this possible. 😘
