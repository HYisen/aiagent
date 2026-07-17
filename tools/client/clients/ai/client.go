package ai

import (
	"aiagent/clients/model"
	"aiagent/clients/openai"
	"aiagent/helpers/closer"
	"aiagent/service/chat"
	"aiagent/tools/client/keeper"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"

	"github.com/hyisen/wf"
)

var ErrForbidden = errors.New("server responds 403")

type V1Client struct {
	endpoint  string
	loginFunc LoginFunc
}

func (c *V1Client) GetSession(id int) (model.Session, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/sessions/%d", c.endpoint, id), nil)
	if err != nil {
		return model.Session{}, err
	}
	return FetchAndParseJSON[model.Session](req)
}

type v1Session struct {
	ID int
	SessionWithoutID
}

func (s v1Session) IDValue() int {
	return s.ID
}

func (s v1Session) IDField() string {
	return "ID"
}

func (s v1Session) SessionCommon() SessionWithoutID {
	return s.SessionWithoutID
}

func (c *V1Client) ListSessions() ([]Session, error) {
	req, err := http.NewRequest(http.MethodGet, c.endpoint+"/v1/sessions", nil)
	if err != nil {
		return nil, err
	}
	items, err := FetchAndParseJSON[[]v1Session](req)
	if err != nil {
		return nil, err
	}
	return castUp(items), nil
}

func NewClient(endpoint string, loginFunc LoginFunc) *V1Client {
	return &V1Client{
		endpoint:  endpoint,
		loginFunc: loginFunc,
	}
}

type LoginFunc func() (username string, password string, err error)

func (c *V1Client) UpgradeOptional() (neo Client, err error) {
	username, password, err := c.loginFunc()
	if err != nil {
		return nil, err
	}

	credential := keeper.Credential{
		Username: username,
		Password: password,
	}
	k := keeper.New(credential, c.serverHost())

	ctx, cancelFunc := context.WithTimeout(context.Background(), keeper.GenerateTimeout)
	defer cancelFunc()
	token, err := keeper.GenerateToken(ctx, c.serverHost(), credential)
	if err != nil {
		return nil, err
	}

	neo = NewV2Client(c.endpoint, k, token.UserID)
	return neo, nil
}

func (c *V1Client) serverHost() string {
	u, err := url.Parse(c.endpoint)
	if err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
}

func (c *V1Client) CreateSession() (id int, err error) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/v1/sessions", c.endpoint), nil)
	if err != nil {
		return 0, err
	}
	return FetchAndParseJSON[int](req)
}

func newChatRequest(url string, content string) (*http.Request, error) {
	payload := &chat.RequestPayload{
		Content: content,
		Model:   string(openai.ChatModelDeepSeekV4Pro),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", wf.JSONContentType)
	return req, nil
}

func doAndHandleResponse(req *http.Request) (words <-chan string, err error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		closer.CloseAndWarnIfFail(resp.Body)
		return nil, fmt.Errorf("bad status %v", resp.Status)
	}

	ch := make(chan string)
	// Pass owner of body to goroutine, DO NOT close it here. Don't ask me how I know it.
	go transform(resp.Body, ch)
	return ch, nil
}

func (c *V1Client) Chat(sessionID int, content string) (words <-chan string, err error) {
	req, err := newChatRequest(fmt.Sprintf("%s/v1/sessions/%d/chat?stream=true", c.endpoint, sessionID), content)
	if err != nil {
		return nil, err
	}
	return doAndHandleResponse(req)
}

func (c *V1Client) GetVersion() (version *debug.BuildInfo, err error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/build-info", c.endpoint), nil)
	if err != nil {
		return nil, err
	}

	v, err := FetchAndParseJSON[debug.BuildInfo](req)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (c *V1Client) GenerateSessionName(cmd string) (null map[int]string, err error) {
	id, err := strconv.Atoi(cmd)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/v1/sessions/%d/name/generate", c.endpoint, id), nil)
	if err != nil {
		return nil, err
	}

	_, err = Fetch(req)
	return nil, err
}
