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
	data, err := doRequestAndVerifyStatusReadBodyAll(request)
	if err != nil {
		return ret, err
	}

	if err := json.Unmarshal(data, &ret); err != nil {
		return ret, err
	}
	return ret, nil
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
