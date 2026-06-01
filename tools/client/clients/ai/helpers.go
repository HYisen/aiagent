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

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return ret, err
	}
	defer func(c io.Closer) {
		err = errors.Join(err, c.Close())
	}(resp.Body)

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ret, err
	}
	if resp.StatusCode == http.StatusForbidden {
		return ret, fmt.Errorf("%w: %s", ErrForbidden, string(data))
	}
	if resp.StatusCode != http.StatusOK {
		return ret, fmt.Errorf("unexpected response status %v: %s", resp.Status, string(data))
	}

	if err := json.Unmarshal(data, &ret); err != nil {
		return ret, err
	}
	return ret, nil
}
