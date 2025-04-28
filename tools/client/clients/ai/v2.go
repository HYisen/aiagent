package ai

import (
	"aiagent/clients/model"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

type TokenProvider interface {
	GetToken() (string, error)
}

type V2Client struct {
	endpoint      string
	tokenProvider TokenProvider
	userID        int
}

func NewV2Client(endpoint string, tokenProvider TokenProvider, userID int) *V2Client {
	return &V2Client{endpoint: endpoint, tokenProvider: tokenProvider, userID: userID}
}

func (c *V2Client) UpgradeOptional() (neo Client, err error) {
	return nil, nil
}

func (c *V2Client) CreateSession() (id int, error error) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/v2/users/%d/sessions", c.endpoint, c.userID), nil)
	if err != nil {
		return 0, err
	}

	c.AttachToken(req)

	data, err := doRequestAndHandleResponse(req)
	if err != nil {
		return 0, err
	}

	var session model.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return 0, err
	}
	return session.ScopedID, nil
}

func (c *V2Client) Chat(sessionScopedID int, content string) (words <-chan string, err error) {
	req, err := newChatRequest(fmt.Sprintf(
		"%s/v2/users/%d/sessions/%d/chat?stream=true",
		c.endpoint,
		c.userID,
		sessionScopedID,
	), content)
	if err != nil {
		return nil, err
	}
	c.AttachToken(req)
	return doAndHandleResponse(req)
}

// AttachToken add token header to req with token from c.tokenProvider.
// This behavior shall be privileged and limited, thus I drop the other path that
// features Client with an http.Client which has http.RoundTripper that automatically gets and attaches token.
func (c *V2Client) AttachToken(req *http.Request) {
	token, err := c.tokenProvider.GetToken()
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Token", token)
}

func (c *V1Client) serverHost() string {
	u, err := url.Parse(c.endpoint)
	if err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
}
