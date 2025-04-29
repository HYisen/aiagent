package ai

import (
	"aiagent/clients/openai"
	"aiagent/service/chat"
	"aiagent/tools/client/keeper"
	"aiagent/tools/client/ui"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hyisen/wf"
	"io"
	"net/http"
	"strconv"
)

var ErrForbidden = errors.New("server responds 403")

type V1Client struct {
	endpoint string
}

func VerifyStatusReadBodyOnFail(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unexpected http status code %d no body %v", resp.StatusCode, err)
	}
	return fmt.Errorf("unexpected http status code %d: %s", resp.StatusCode, string(data))
}

func verifyParseExtract[ItemType Session](resp *http.Response) (idToName map[int]string, err error) {
	if err := VerifyStatusReadBodyOnFail(resp); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var items []ItemType
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}

	idToName = make(map[int]string)
	for _, item := range items {
		idToName[item.Key()] = item.Value()
	}
	return idToName, nil
}

type v1Session struct {
	ID   int
	Name string
}

func (s v1Session) Key() int {
	return s.ID
}

func (s v1Session) Value() string {
	return s.Name
}

func (c *V1Client) ListSessions() (map[int]string, error) {
	resp, err := http.Get(c.endpoint + "/v1/sessions")
	if err != nil {
		return nil, err
	}
	defer openai.CloseAndWarnIfFail(resp.Body)

	return verifyParseExtract[v1Session](resp)
}

func NewClient(endpoint string) *V1Client {
	return &V1Client{
		endpoint: endpoint,
	}
}

func (c *V1Client) UpgradeOptional() (neo Client, err error) {
	username, password, err := ui.Login()
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

func doRequestAndHandleResponse(req *http.Request) (responsePayload []byte, err error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(c io.Closer) {
		err = errors.Join(err, c.Close())
	}(resp.Body)

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
	data, err := doRequestAndHandleResponse(req)
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
		Model:   openai.ChatModelDeepSeekR1,
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
