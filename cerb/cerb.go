package cerb

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

// CerberusTicketSearchResults is the raw structure returned by the Cerberus search API when looking for tickets. Most often you want to call a function that hides all these details and work with a []CerberusTicket instead.
type CerberusTicketSearchResults struct {
	Status  string           `json:"__status"`
	Count   int              `json:"count"`
	Limit   int              `json:"limit"`
	Page    int              `json:"page"`
	Results []CerberusTicket `json:"results"`
	Total   int              `json:"total"`
	Version string           `json:"__version"`
}

// CerberusCreds contains the keys needed to connect to the Cerberus API. @see https://cerb.ai/docs/api/authentication/
type CerberusCreds struct {
	Key    string `json:"access-key"`
	Secret string `json:"access-secret"`
}

// Cerberus handles all the interaction with the Cerb API.
type Cerberus struct {
	creds          CerberusCreds
	client         http.Client
	restAPIBaseURL string
}

// NewCerberus create a new Cerberus
func NewCerberus(creds CerberusCreds, client http.Client, baseURL string) Cerberus {
	c := Cerberus{
		creds:          creds,
		client:         client,
		restAPIBaseURL: baseURL,
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
