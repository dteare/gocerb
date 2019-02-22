package cerb

import (
	"bytes"
	_md5 "crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The Cerb api requires all parameters and payloads to be hashed on the client side and this hash needs to match the hash performed by the server **exactly**. All parameters must be sorted and encoded.
//
// @see https://cerb.ai/docs/api/authentication/ for details.

func (c Cerberus) performRequest(method string, endpoint string, params url.Values, form url.Values, target interface{}) error {
	location, _ := time.LoadLocation("GMT")
	t := time.Now().In(location)
	date := t.Format(time.RFC1123)
	req, err := http.NewRequest(method, c.creds.RestAPIBaseURL+endpoint, strings.NewReader(form.Encode()))

	if len(params) > 0 {
		q := req.URL.Query()

		for key, value := range params {
			q.Add(key, value[0])
		}
		req.URL.RawQuery = q.Encode()
	}

	if err != nil {
		return fmt.Errorf("Error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Date", date)

	signature := generateSignature(req, c.creds.Secret)
	req.Header.Set("Cerb-Auth", c.creds.Key+":"+signature)

	resp, err := c.client.Do(req)

	if err != nil {
		return fmt.Errorf("Error performing %s request on %s: %v", method, endpoint, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error status code of %d returned when performing %s request on %s", resp.StatusCode, method, endpoint)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response body: %v", err)
	}
	resp.Body.Close()

	// fmt.Printf("Response JSON from %s:\n%s\n\n", endpoint, string(body))

	// 💥 Some end points will return status 200 and set the body to {"__status":"error"} along with an explaination in the message key. Instead of having callers worry about this we do our best to fix that here.
	err = extractErrorFromJSONBody(body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, target)
	if err != nil {
		return fmt.Errorf("Error decoding response body: %v", err)
	}

	resp.Body.Close()
	return nil
}

func generateSignature(req *http.Request, secret string) string {
	body, err := ioutil.ReadAll(req.Body)

	if err != nil {
		return ""
	}

	s := req.Method + "\n" +
		req.Header.Get("Date") + "\n" +
		req.URL.Path + "\n" +
		req.URL.Query().Encode() + "\n" +
		string(body) + "\n" +
		md5(secret) + "\n"

	req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	signature := md5(s)

	return signature
}

func md5(s string) string {
	h := _md5.New()
	io.WriteString(h, s)
	bytes := h.Sum(nil)

	dst := make([]byte, hex.EncodedLen(len(bytes)))
	hex.Encode(dst, bytes)

	return string(dst)
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
		return fmt.Errorf("Unable to parse response body to extract the error: %v", err)
	}

	return fmt.Errorf("Response body contained non-success status of %s: %v", resp.Status, resp.Message)
}
