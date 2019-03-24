package cerb

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CerberusCreds contains the keys needed to connect to the Cerberus API. @see https://cerb.ai/docs/api/authentication/
type CerberusCreds struct {
	Key            string `json:"access-key"`
	Secret         string `json:"access-secret"`
	RestAPIBaseURL string `json:"restAPIBaseURL"`
}

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

// CerberusTicket models the ticket object used by Cerb
type CerberusTicket struct {
	BucketID    int    `json:"bucket_id"`
	Email       string `json:"initial_message_sender_email"` // Only set when `initial_message_sender_` is expanded
	GroupID     int    `json:"group_id"`
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

// CreateCommentResponse represents the response from the records/comment/create.json endpoint
type CreateCommentResponse struct {
	ID           int
	CreatedAt    int    `json:"created"`
	Importance   int    `json:"importance"`
	Mask         string `json:"mask"`
	MessageCount string `json:"num_messages"`
	Status       string `json:"status"`
	Subject      string `json:"subject"`
	URL          string `json:"url"`
}

// CustomerQuestion represents a question asked by a user that needs to be created as a Ticket in Cerb. Additional fields allow you to control where to create the ticket, notes to add, initial status, etc.
type CustomerQuestion struct {
	BucketID int
	GroupID  int

	To           string
	From         string
	Participants []string
	Subject      string
	Content      string

	CustomFields []CustomField
	Notes        string
	Status       string // [o]pen, [c]losed, [w]aiting. Defaults to [o]
}

// CustomField allows records in Cerb can be extended with custom fields. @see https://cerb.ai/docs/api/topics/custom-fields/
type CustomField struct {
	ID    int
	Value string
}

// SetCustomTicketFieldsResponse is the response from the PUT records/tickets/123.json endpoint
type SetCustomTicketFieldsResponse struct {
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
	status := q.Status
	if status == "" {
		status = "o"
	}
	participants := strings.Join(q.Participants, ", ")
	form := url.Values{}
	form.Set("fields[group_id]", strconv.Itoa(q.GroupID))
	form.Set("fields[bucket_id]", strconv.Itoa(q.BucketID))
	form.Set("fields[status]", status)
	form.Set("fields[subject]", q.Subject)
	form.Set("fields[participants]", participants)

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
		return nil, fmt.Errorf("Failed to create Cerberus message on ticket %d: %v", ticket.ID, err)
	}

	c.SetCustomTicketFields(ticket.ID, q.CustomFields)

	if q.Notes != "" {
		err = c.CreateComment(ticket.ID, q.Notes)

		if err != nil {
			return nil, fmt.Errorf("Failed to create comment on ticket %d: %v", ticket.ID, err)
		}
	}

	return &message, nil
}

// CreateComment adds a comment to an existing ticket
func (c Cerberus) CreateComment(ticketID int, comment string) error {
	params := url.Values{}
	params.Set("fields[author__context]", "ticket")
	params.Set("fields[author_id]", strconv.Itoa(17))
	params.Set("fields[comment]", comment)
	params.Set("fields[target__context]", "ticket")
	params.Set("fields[target_id]", strconv.Itoa(ticketID))

	var ticket CreateCommentResponse
	fmt.Println("CREATING COMMENTS")
	err := c.performRequest(http.MethodPost, "records/comment/create.json", params, nil, &ticket)

	if err != nil {
		return fmt.Errorf("Failed to create ticket comment: %v", err)
	}

	return nil
}

// SetCustomTicketFields updates the custom fields for the given ticket.
func (c Cerberus) SetCustomTicketFields(ticketID int, customFields []CustomField) error {
	for _, cf := range customFields {
		var update SetCustomTicketFieldsResponse

		form := url.Values{}
		form.Set("fields[custom_"+strconv.Itoa(cf.ID)+"]", cf.Value)

		err := c.performRequest(http.MethodPut, "records/tickets/"+strconv.Itoa(ticketID)+".json?expand=custom_", nil, form, &update)

		if err != nil {
			return fmt.Errorf("Error setting custom ticket field %d to %v: %v", cf.ID, cf.Value, err)
		}
	}

	return nil
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

// SearchGroupResponse is the response from the records/group/search.json endpoint
type SearchGroupResponse struct {
	Status  string  `json:"__status"`
	Count   int     `json:"count"`
	Limit   int     `json:"limit"`
	Page    int     `json:"page"`
	Results []Group `json:"results"`
	Total   int     `json:"total"`
	Version string  `json:"__version"`
}

// SearchBucketsResponse is the response from the records/bucket/search.json endpoint
type SearchBucketsResponse struct {
	Status  string   `json:"__status"`
	Count   int      `json:"count"`
	Limit   int      `json:"limit"`
	Page    int      `json:"page"`
	Results []Bucket `json:"results"`
	Total   int      `json:"total"`
	Version string   `json:"__version"`
}

// Group represents a group within Cerb records/bucket/search.json endpoint
type Group struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"record_url"`

	Updated int `json:"updated"`
	Created int `json:"created"`

	Buckets []Bucket
}

// SearchBucketsResponse is the result from the

// Bucket represents a bucket within Cerb
type Bucket struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"record_url"`

	Updated int `json:"updated_at"`
	Default int `json:"is_default"`

	GroupID   int    `json:"group_id"`
	GroupName string `json:"group_name"`
}

// FindAllGroups searches for all groups
func (c Cerberus) FindAllGroups() (*[]Group, error) {
	limit := 250 // If you need pagination imitate ListOpenTickets
	params := url.Values{}
	params.Set("q", "")
	params.Set("limit", strconv.Itoa(limit))

	var r SearchGroupResponse
	err := c.performRequest(http.MethodGet, "records/group/search.json", params, nil, &r)

	if err != nil {
		return nil, fmt.Errorf("ListGroups failed to search groups: %v", err)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshall ListGroups search results: %v", err)
	}

	return &r.Results, nil
}

// FindAllBuckets will search Cerb for buckets
func (c Cerberus) FindAllBuckets() (*[]Bucket, error) {
	limit := 250 // If you need pagination imitate ListOpenTickets
	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))
	params.Set("expand", "group_")

	var r SearchBucketsResponse
	err := c.performRequest(http.MethodGet, "records/bucket/search.json", params, nil, &r)

	if err != nil {
		return nil, fmt.Errorf("ListGroups failed to search buckets: %v", err)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshall FindBuckets search results: %v", err)
	}

	return &r.Results, nil
}

// FindBucketsInGroup will search Cerb for buckets within the given group
func (c Cerberus) FindBucketsInGroup(groupID int) (*[]Bucket, error) {
	limit := 250 // If you need pagination imitate ListOpenTickets
	params := url.Values{}
	params.Set("q", "group.id:["+strconv.Itoa(groupID)+"]")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("expand", "group_")

	var r SearchBucketsResponse
	err := c.performRequest(http.MethodGet, "records/bucket/search.json", params, nil, &r)

	if err != nil {
		return nil, fmt.Errorf("ListGroups failed to search buckets: %v", err)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshall FindBuckets search results: %v", err)
	}

	return &r.Results, nil
}

// FindAllGroupsAndBuckets searches Cerb for all Groups and the Buckets within them.
func (c Cerberus) FindAllGroupsAndBuckets() (*[]Group, error) {
	groups, err := c.FindAllGroups()

	if err != nil {
		return nil, fmt.Errorf("error listing groups: %v", err)
	}

	for i, group := range *groups {
		buckets, err := c.FindBucketsInGroup(group.ID)

		if err != nil {
			return nil, fmt.Errorf("AllBucketsByGroup failed to find buckets in group %d: %v", group.ID, err)
		}

		for j := range *buckets {
			(*buckets)[j].GroupName = group.Name
		}

		(*groups)[i].Buckets = *buckets
	}

	return groups, nil
}
