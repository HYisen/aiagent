package ai

import (
	"aiagent/clients/model"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
)

type TokenProvider interface {
	GetToken() (string, error)
}

type V2Client struct {
	endpoint      string
	tokenProvider TokenProvider
	userID        int
}

func (c *V2Client) GetSession(id int) (model.Session, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v2/users/%d/sessions/%d", c.endpoint, c.userID, id), nil)
	if err != nil {
		return model.Session{}, err
	}
	c.AttachToken(req)
	return FetchAndParseJSON[model.Session](req)
}

func (c *V2Client) ListSessions() ([]Session, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v2/users/%d/sessions", c.endpoint, c.userID), nil)
	if err != nil {
		return nil, err
	}

	c.AttachToken(req)

	items, err := FetchAndParseJSON[[]v2Session](req)
	if err != nil {
		return nil, err
	}
	return castUp(items), err
}

type v2Session struct {
	ScopedID int
	SessionWithoutID
}

func (s v2Session) IDValue() int {
	return s.ScopedID
}

func (s v2Session) IDField() string {
	return "ScopedID"
}

func (s v2Session) SessionCommon() SessionWithoutID {
	return s.SessionWithoutID
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

	session, err := FetchAndParseJSON[model.Session](req)
	if err != nil {
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

func (c *V2Client) GetVersion() (version *debug.BuildInfo, err error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/build-info", c.endpoint), nil)
	if err != nil {
		return nil, err
	}
	c.AttachToken(req)
	v, err := FetchAndParseJSON[debug.BuildInfo](req)
	return &v, err
}

func (c *V2Client) GenerateSessionName(cmd string) (scopedIDToNeoName map[int]string, err error) {
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/v2/users/%d/sessions/name/generate", c.endpoint, c.userID),
		strings.NewReader(cmd),
	)
	if err != nil {
		return nil, err
	}

	return FetchAndParseJSON[map[int]string](req)
}
