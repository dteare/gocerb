package cerb

// 💥 BEWARE: the Cerb api requires all paramters and payloads to be hashed on the client side and this hash needs to match the hash performed by the server **exactly**. All parameters must be sorted and encoded. The url encoding chosen by Cerb is different than a straight up url encoding so you must be extremely careful.
//
// The standard library url functions available don't match what Cerb is expecting. They'll also introduce subtle bugs by replacing `'` with `"` in some situations, which will cause the hash to be computed incorrectly as the server uses `'`.
//
// Long story short, this implementation is a complete hack job and brittle af. Be careful and test frequently!

import (
	"bytes"
	_md5 "crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// CerberusTicket models the ticket object used by Cerb
type CerberusTicket struct {
	BucketID    int    `json:"bucket_id"`
	ID          int    `json:"id"`
	Mask        string `json:"mask"`
	NumMessages string `json:"num_messages"`
	Subject     string `json:"subject"`
	Status      string `json:"status"`
	URL         string `json:"url"`
}

// CerberusTicketSearchResults is the raw structure returned by the Cerberus search API when looking for tickets. Note that the format can change based on the search critera so some fields must be an interface{} and be carefully cast. 😕
type CerberusTicketSearchResults struct {
	Status  string           `json:"__status"`
	Count   int              `json:"count"`
	Limit   int              `json:"limit"`
	Page    int              `json:"page"`
	Results []CerberusTicket `json:"results"`
	Total   interface{}      `json:"total"` // Want int but Cerb will randomly return a string
	Version string           `json:"__version"`
}

// CerberusCreateTicketRequest is the format used by clients to create a new Cerb ticket
type CerberusCreateTicketRequest struct {
	Bucket  string `json:"bucket"`
	Subject string `json:"subject"`
	Content string `json:"content"`
	To      string `json:"to"`
	From    string `json:"from"`
	Status  string `json:"status"` // 0=open, 1=waiting, 2=closed, 3=deleted
}

// CerberusCreds contains the keys needed to connect to the Cerberus API. @see https://cerb.ai/docs/api/authentication/
type CerberusCreds struct {
	Key    string `json:"access-key"`
	Secret string `json:"access-secret"`
}

var baseURL = "https://agilebits.cerb.me"

// Cerberus handles all the interaction with the Cerb API.
type Cerberus struct {
	creds  CerberusCreds
	client http.Client
}

// NewCerberus create a new Cerberus
func NewCerberus(creds CerberusCreds, client http.Client) Cerberus {
	c := Cerberus{
		creds:  creds,
		client: client,
	}
	return c
}

// FindTicketsByEmail finds all open tickets for the given email address.
func (c Cerberus) FindTicketsByEmail(email string) (*[]CerberusTicket, error) {
	method := "GET"
	payload := ""
	path := "/rest/records/ticket/search.json"
	query := cerbEncode("q=status:[o] messages.first:(sender:(email:'" + email + "'))")

	headers := reqHeaders(c.creds, method, path, query, payload)

	resp, err := c.performCerbRequest(http.MethodGet, baseURL+path+"?"+query, nil, headers)

	if err != nil {
		fmt.Printf("Failed to perform Cerb request: %v", err)
		return nil, err
	}

	var cerbResp CerberusTicketSearchResults
	err = json.Unmarshal(resp, &cerbResp)

	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshall cerb search results: %v", err)
	}

	return &cerbResp.Results, nil
}

// ListOpenTickets finds all open tickets in Cerberus. The Cerb api returns things grouped by pages so the caller needs to specify which page they want. The returned bool indicates if there are more pages remaining to load.
func (c Cerberus) ListOpenTickets(page int) (*[]CerberusTicket, bool, error) {
	limit := 100
	method := "GET"
	payload := ""
	path := "/rest/records/ticket/search.json"
	query := cerbEncode(fmt.Sprintf("q=status:[o] group:(Billing OR Sales) page:%d limit:%d", page, limit))

	headers := reqHeaders(c.creds, method, path, query, payload)

	resp, err := c.performCerbRequest(http.MethodGet, baseURL+path+"?"+query, nil, headers)

	if err != nil {
		fmt.Printf("Failed to perform Cerb request: %v", err)
		return nil, false, err
	}

	fmt.Println("Body:\n", string(resp))

	var results CerberusTicketSearchResults
	err = json.Unmarshal(resp, &results)

	total, ok := results.Total.(int)
	if !ok {
		tstr, ok := results.Total.(string)
		if ok {
			total, err = strconv.Atoi(tstr)

			if err != nil {
				panic("Failed to parse the results total as a string:" + tstr)
			}
		} else {
			panic(reflect.TypeOf(results.Total))
		}
	}

	remaining := total - ((page + 1) * limit) // Page and Limit in the response are incorrect
	// fmt.Printf("Total of %d tickets w/ %d limit\n", total, limit)
	fmt.Printf("Loaded %d tickets from page %d. %d tickets remain on subseqent pages.\n", results.Count, page, remaining)

	if err != nil {
		return nil, false, fmt.Errorf("Failed to unmarshall cerb search results: %v", err)
	}

	return &results.Results, remaining > 0, nil
}

// CreateTicket creates a ticket in Cerberus
func (c Cerberus) CreateTicket(message string) (*CerberusTicket, error) {
	method := "POST"
	path := "/rest/parser/parse.json"
	query := ""
	payload := "message=" + cerbEncode(message)
	headers := reqHeaders(c.creds, method, path, query, payload)
	resp, err := c.performCerbRequest(http.MethodPost, baseURL+path, strings.NewReader(payload), headers)

	if err != nil {
		return nil, err
	}

	var cerbResp interface{}
	err = json.Unmarshal(resp, &cerbResp)

	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshall create cerb ticket response: %v", err)
	}

	m := cerbResp.(map[string]interface{})
	status := m["__status"].(string)

	if status != "success" {
		return nil, fmt.Errorf("Unsuccessful status returned by create Cerb ticket api: %s", string(resp))
	}

	r := CerberusTicket{
		ID: int(m["id"].(float64)),
		// BucketID:    int(m["bucket_id"].(float64)),
		Mask: m["mask"].(string),
		// Subject: m["subject"].(string),
		// Status: m["status"].(string),
		// NumMessages: m["num_messages"].(string),
	}

	return &r, nil
}

// Create the headers needed by Cerberus, including auth. This is fragile af. Be sure you:
//   💥 Sort your parameters yourself
//   💥 Encode the query and payload using the brain dead `cerbEncode`; anything smarter will give an authorization failure
func reqHeaders(creds CerberusCreds, method string, path string, query string, payload string) http.Header {
	location, _ := time.LoadLocation("GMT")
	t := time.Now().In(location)
	date := t.Format(time.RFC1123)

	var stringToSign = method + "\n" + date + "\n" + path + "\n" + query + "\n" + payload + "\n" + md5(creds.Secret) + "\n"

	return http.Header{
		"Date":         []string{date},
		"Cerb-Auth":    []string{creds.Key + ":" + md5(stringToSign)},
		"encoding":     []string{"UTF-8"},
		"Content-Type": []string{"application/x-www-form-urlencoded"},
	}
}

func cerbEncode(s string) string {
	// Cerb only encodes certain characters server side so we can't use this ☹️
	// encodedQuery := u.Query().Encode()

	// Selectively encode random things until it works...
	s = strings.Replace(s, "'", "%22", -1)
	s = strings.Replace(s, ",", "%2C", -1)
	s = strings.Replace(s, " ", "%20", -1)

	return s
}

func md5(s string) string {
	h := _md5.New()
	io.WriteString(h, s)
	bytes := h.Sum(nil)

	dst := make([]byte, hex.EncodedLen(len(bytes)))
	hex.Encode(dst, bytes)

	return string(dst)
}

func (c Cerberus) performCerbRequest(method string, urlString string, body io.Reader, headers http.Header) ([]byte, error) {
	req, err := http.NewRequest(method, urlString, body)

	if err != nil {
		return nil, fmt.Errorf("Failed to create request: %v", err)
	}

	for key, header := range headers {
		req.Header.Set(key, header[0])
	}
	resp, err := c.client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("Client refused to do their work! %v", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Cerberus gave us an error status: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("Failed to read body: %v", err)
	}

	// 💥 BEWARE that cerb response status can NOT be trusted. End points will return status 200 and set the body to {"__status":"error"} along with an explaination in the message key. Instead of having callers worry about this we do our best to fix that here.
	err = extractErrorFromJSONBody(b)

	if err != nil {
		return nil, err
	}

	return b, nil
}

// CerberusErrorResponse is the structure returned by Cerb when there is a server error. A status code of 200 is used by the response so we need to parse this out and handle it ourselves.
// i.e. response:
//		StatusCode 200
//		Body {"__status":"error","message":"Access denied! (Invalid credentials: access key)"}
type CerberusErrorResponse struct {
	Status  string `json:"__status"`
	Message string `json:"message"`
}

func extractErrorFromJSONBody(b []byte) error {
	if bytes.Contains(b, []byte(`"__status":"success"`)) {
		return nil
	}

	var resp CerberusErrorResponse
	err := json.Unmarshal(b, &resp)

	if err != nil {
		fmt.Printf("Unable to parse response body to extract the error:\n\t%v\n%s", err, string(b))
		return err
	}

	return fmt.Errorf("Response body contained non-success status of %s: %v", resp.Status, resp.Message)
}
