package cerb

// 💥 BEWARE: the Cerb api requires all paramters and payloads to be hashed on the client side and this hash needs to match the hash performed by the server **exactly**. All parameters must be sorted and encoded. The url encoding chosen by Cerb is different than a straight up url encoding so you must be extremely careful.
//
// The standard library url functions available don't match what Cerb is expecting. They'll also introduce subtle bugs by replacing `'` with `"` in some situations, which will cause the hash to be computed incorrectly as the server uses `'`.
//
// Long story short, this implementation is a complete hack job and brittle af. Be careful and test frequently!

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// CerberusTicket models the ticket object used by Cerb
type CerberusTicket struct {
	BucketID    int    `json:"bucket_id"`
	Email       string `json:"initial_message_sender_email"` // Only set when `initial_message_sender_` is expanded
	ID          int    `json:"id"`
	Mask        string `json:"mask"`
	NumMessages string `json:"num_messages"`
	Subject     string `json:"subject"`
	Status      string `json:"status"`
	URL         string `json:"url"`
}

// CerberusTicketSearchResults is the raw structure returned by the Cerberus search API when looking for tickets.
type CerberusTicketSearchResults struct {
	Status  string           `json:"__status"`
	Count   int              `json:"count"`
	Limit   int              `json:"limit"`
	Page    int              `json:"page"`
	Results []CerberusTicket `json:"results"`
	Total   int              `json:"total"`
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
var restAPIBaseURL = "https://agilebits.cerb.me/rest/"

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

// CustomerQuestion represents a question asked by a user that needs to be created as a Ticket in Cerb. Additional fields allow you to control where to create the ticket, notes to add, initial state, etc.
type CustomerQuestion struct {
	BucketID int
	GroupID  int

	To      string
	From    string
	Subject string
	Content string

	Notes string
}

// CreateTicketResponse represents the response from the records/ticket/create.json endpoint
type CreateTicketResponse struct {
	ID           int
	CreatedAt    int    `json:"created"`
	Importance   int    `json:"importance"`
	Mask         string `json:"mask"`
	MessageCount string `json:"num_messages"`
	Status       string `json:"status"`
	Subject      string `json:"subject"`
	URL          string `json:"url"`
}

// CreateMessageResponse represents the response from the records/message/create.json endpoint
type CreateMessageResponse struct {
	ID       int `json:"id"`
	SenderID int `json:"sender_id"`

	InitialMessageSenderAvatar    string `json:"ticket_initial_message_sender__image_url"`
	InitialMessageSenderEmail     string `json:"ticket_initial_message_sender_email"`
	InitalMessageSenderID         int    `json:"ticket_initial_message_sender_id"`
	InitialMessageURL             string `json:"ticket_initial_message_record_url"`
	InitialMessageSenderRecordURL string `json:"ticket_initial_message_sender_record_url"`

	TicketID      int    `json:"ticket_initial_message_ticket_id"`
	TicketLabel   string `json:"ticket__label"`
	TicketMask    string `json:"ticket_mask"`
	TicketStatus  string `json:"ticket_status"`
	TicketSubject string `json:"ticket_subject"`
	TicketURL     string `json:"ticket_url"`
}

// CreateMessage uses the Cerb api to create a new ticket
func (c Cerberus) CreateMessage(q CustomerQuestion) (*CreateMessageResponse, error) {
	// Create a ticket (a "thread" that will contain the messages for this conversation)
	form := url.Values{}
	form.Set("fields[group_id]", strconv.Itoa(q.GroupID))
	form.Set("fields[bucket_id]", strconv.Itoa(q.BucketID))
	form.Set("fields[subject]", q.Subject)
	form.Set("fields[participants]", "customer@example.com")

	var ticket CreateTicketResponse
	err := c.performRequest(http.MethodPost, "records/ticket/create.json", nil, form, &ticket)

	if err != nil {
		return nil, fmt.Errorf("Failed to create Cerberus ticket: %v", err)
	}

	// Create a message on the newly created ticket
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s", q.From, q.To, q.Subject)
	form = url.Values{}
	form.Set("expand", "ticket_initial_message_sender_")
	form.Set("fields[ticket_id]", strconv.Itoa(ticket.ID))
	form.Set("fields[sender]", q.From)
	form.Set("fields[headers]", headers)
	form.Set("fields[content]", q.Content)

	var message CreateMessageResponse
	err = c.performRequest(http.MethodPost, "records/message/create.json", nil, form, &message)

	if err != nil {
		return nil, fmt.Errorf("Failed to create Cerberus ticket: %v", err)
	}

	return &message, nil
}

// FindTicketsByEmail finds all open tickets for the given email address.
func (c Cerberus) FindTicketsByEmail(email string) (*[]CerberusTicket, error) {
	params := url.Values{}
	params.Set("q", "status:[o] messages.first:(sender:(email:"+email+"))")

	var r CerberusTicketSearchResults
	err := c.performRequest(http.MethodGet, "records/ticket/search.json", params, nil, &r)

	if err != nil {
		return nil, fmt.Errorf("Failed to search tickets: %v", err)
	}

	return &r.Results, nil
}

// ListOpenTickets finds all open tickets in Cerberus. The Cerb api returns things grouped by pages so the caller needs to specify which page they want. Returns the first page of matching tickets and the number of additional tickets remaining on subsequent pages.
func (c Cerberus) ListOpenTickets(page int) (*[]CerberusTicket, int, error) {
	limit := 100 // Maximum of 250 enforced by server
	params := url.Values{}
	params.Set("q", "status:[o]")
	params.Set("page", strconv.Itoa(page))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("expand", "initial_message_sender_")

	var r CerberusTicketSearchResults
	err := c.performRequest(http.MethodGet, "records/ticket/search.json", params, nil, &r)

	if err != nil {
		return nil, 0, fmt.Errorf("ListOpenTickets failed to search tickets: %v", err)
	}

	remaining := r.Total - ((page + 1) * limit) // Page and Limit in response are incorrect

	if err != nil {
		return nil, 0, fmt.Errorf("Failed to unmarshall cerb search results: %v", err)
	}

	if remaining < 0 {
		remaining = 0
	}
	return &r.Results, remaining, nil
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
