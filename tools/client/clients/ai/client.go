package ai

import (
	"aiagent/clients/openai"
	"aiagent/helpers/closer"
	"aiagent/service/chat"
	"aiagent/tools/client/keeper"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	resp, err := http.Get(c.endpoint + "/v1/sessions")
	if err != nil {
		return nil, err
	}
	defer closer.CloseAndWarnIfFail(resp.Body)

	data, err := VerifyStatusReadBodyAll(resp)
	if err != nil {
		return nil, err
	}

	var items []v1Session
	if err := json.Unmarshal(data, &items); err != nil {
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

func doRequestAndVerifyStatusReadBodyAll(req *http.Request) (responsePayload []byte, err error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(c io.Closer) {
		err = errors.Join(err, c.Close())
	}(resp.Body)
	return VerifyStatusReadBodyAll(resp)
}

func VerifyStatusReadBodyAll(resp *http.Response) (responsePayload []byte, err error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w: %s", ErrForbidden, string(data))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status code %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

func (c *V1Client) CreateSession() (id int, error error) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/v1/sessions", c.endpoint), nil)
	if err != nil {
		return 0, err
	}
	data, err := doRequestAndVerifyStatusReadBodyAll(req)
	if err != nil {
		return 0, err
	}
	num, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}
	return num, nil
}

func newChatRequest(url string, content string) (*http.Request, error) {
	payload := &chat.RequestPayload{
		Content: content,
		Model:   openai.ChatModelDeepSeekV4Pro,
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
		return nil, fmt.Errorf("bad status %d %s", resp.StatusCode, resp.Status)
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
	resp, err := http.Get(fmt.Sprintf("%s/v1/build-info", c.endpoint))
	if err != nil {
		return nil, err
	}
	defer closer.CloseAndWarnIfFail(resp.Body)

	data, err := VerifyStatusReadBodyAll(resp)
	if err != nil {
		return nil, err
	}

	version = &debug.BuildInfo{}
	if err := json.Unmarshal(data, version); err != nil {
		return nil, err
	}
	return version, nil
}
