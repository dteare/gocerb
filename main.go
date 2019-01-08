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

	"go.1password.io/sorting-hat/src/cerb"
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

	cerb := cerb.NewCerberus(*creds, *client)
	page := 0

	for {
		tickets, hasMore, err := cerb.ListOpenTickets(page)
		if err != nil {
			fmt.Printf("Error finding open cerb tickets: %v\n", err)
			os.Exit(1)
		}

		for _, t := range *tickets {
			researchTicket(t)
		}

		// Only load first page until things are working
		if !hasMore || 1 == 1 {
			break
		}
		page++
	}
}

func researchTicket(t cerb.CerberusTicket) {
	// TODO: fun things like look up the customer in a CRM, etc.
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
