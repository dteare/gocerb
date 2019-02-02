package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/dteare/gocerb/cerb"
)

var cerbCredsFilepath string

func main() {
	flag.StringVar(&cerbCredsFilepath, "cerb-creds", "~/.config/cerb/creds.json", "Path to file containing Cerb credentials.")
	flag.Parse()

	client := &http.Client{}
	creds, err := loadCerbCreds()

	if err != nil {
		fmt.Printf("Failed to load cerb creds: %v\n", err)
		fmt.Printf("\nSETUP Instructions: Copy sample-creds.json to ~/.config/cerb/creds.json and edit to use your api keys or pass in the -cerb-creds parameter to speficy where to find it.")
		return
	}

	c := cerb.NewCerberus(*creds, *client, "https://agilebits.cerb.me/rest/")
	// testCreateTicket(c)
	// testFindTicketsByEmail(c)
	// testListOpenTickets(c)
	testListGroups(c)
}

func testCreateTicket(c cerb.Cerberus) {
	q := cerb.CustomerQuestion{
		BucketID: 1049,
		GroupID:  900,
		Content:  "Hello there! ❤️",
		From:     "dave+gocerb@1password.com",
		Notes:    "Some exciting notes that stand out in a stunning yellow. 🎨",
		Subject:  "GoCerb! 🤘🏼",
		To:       "support@1password.com",
	}

	m, err := c.CreateMessage(q)
	if err != nil {
		fmt.Printf("Failed to create message: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Create message %d within ticket %d! 💌 %s\n", m.ID, m.ID, m.TicketURL)
}

func testFindTicketsByEmail(c cerb.Cerberus) {
	email := "dave+gocerb@1password.com"
	tickets, err := c.FindTicketsByEmail(email)
	if err != nil {
		fmt.Printf("Failed to find tickets by email: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d open tickets for %s.\n", len(*tickets), email)
}

func testListOpenTickets(c cerb.Cerberus) {
	page := 0

	for {
		tickets, remaining, err := c.ListOpenTickets(page)
		if err != nil {
			fmt.Printf("Error finding open cerb tickets: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Loaded %d tickets from page %d. %d tickets remain on subsequent pages.\n", len(*tickets), page, remaining)

		// for _, t := range *tickets {
		// 	fmt.Println("\t", t.Email)
		// }

		if remaining == 0 {
			break
		}
		page++
	}
}

func testListGroups(c cerb.Cerberus) {
	groups, err := c.FindAllGroupsAndBuckets()

	if err != nil {
		panic(err)
	}

	fmt.Printf("Found %d groups", len(*groups))
}

func loadCerbCreds() (*cerb.CerberusCreds, error) {
	path := cerbCredsFilepath

	if strings.HasPrefix(path, "~/") {
		usr, _ := user.Current()
		home := usr.HomeDir
		path = filepath.Join(home, path[2:])
	}

	bytes, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("Error loading credentials from %s: %v", path, err)
	}

	var creds cerb.CerberusCreds
	err = json.Unmarshal(bytes, &creds)

	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling Cerb creds from %s: %v", path, err)
	}

	return &creds, nil
}
