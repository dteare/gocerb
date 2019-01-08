
# Go Cerb!

Implements the [Cerb API](https://cerb.ai/docs/api/) from Webgroup Media.

## Usage

[Install Go](https://golang.org/doc/install) and then use the sample app to test the api and your setup using:

 `go run main.go`

 You should see the body of the response from GET `/rest/records/tickets/search.json` along with a log like this one:

 > Loaded 1 tickets from page 0. 1015 tickets remain on subseqent pages.

 Note the sample app looks for open tickets in the Billing and Sales groups only.