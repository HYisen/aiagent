package keeper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type LocalKeeper struct {
	token string
}

func NewLocalKeeper(token string) *LocalKeeper {
	return &LocalKeeper{token: token}
}

func (k *LocalKeeper) GetToken() (string, error) {
	return k.token, nil
}

type Keeper struct {
	Token      *Token // Yes, we allow SetToken if you already have one, to reduce GenerateToken frequency.
	credential Credential
	host       string
}

func New(credential Credential, serverHost string) *Keeper {
	return &Keeper{
		credential: credential,
		host:       serverHost,
	}
}

var ExpireTolerance = time.Second * 10 // 10 secs shall be long enough for a world-wide network IO.
var GenerateTimeout = 8 * time.Second  // A bit shorter than ExpireTolerance so that that works.

func (k *Keeper) GetToken() (string, error) {
	if k.Token == nil || k.Token.ExpireAt.Before(time.Now().Add(ExpireTolerance)) {
		ctx, cancelFunc := context.WithTimeout(context.Background(), GenerateTimeout)
		defer cancelFunc()
		token, err := GenerateToken(ctx, k.host, k.credential)
		if err != nil {
			return "", err
		}
		k.Token = token
	}
	return k.Token.ID, nil
}

func GenerateToken(ctx context.Context, host string, credential Credential) (*Token, error) {
	data, err := json.Marshal(credential)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/v1/session", host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(c io.Closer) {
		err = errors.Join(err, c.Close())
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("not ok response status code %d: %s", resp.StatusCode, string(msg))
	}

	var token Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}

type Credential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Token struct {
	ID       string
	ExpireAt time.Time
	Username string
	UserID   int
}
