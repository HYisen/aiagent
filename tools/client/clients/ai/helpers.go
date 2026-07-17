package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

func FetchAndParseJSON[ResponseType any](request *http.Request) (ResponseType, error) {
	var ret ResponseType

	data, err := Fetch(request)
	if err != nil {
		return ret, err
	}

	if err := json.Unmarshal(data, &ret); err != nil {
		return ret, err
	}
	return ret, nil
}

func Fetch(request *http.Request) (payload []byte, err error) {
	resp, err := http.DefaultClient.Do(request)
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
		return nil, fmt.Errorf("unexpected response status %v: %s", resp.Status, string(data))
	}
	return data, nil
}
