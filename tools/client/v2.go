package main

import (
	"aiagent/tools/client/keeper"
	"context"
	"fmt"
	"log"
	"net/url"
)

func (c *Client) upgrade() {
	username, password, err := login()
	if err != nil {
		log.Fatal(err)
	}

	credential := keeper.Credential{
		Username: username,
		Password: password,
	}
	k := keeper.New(credential, c.serverHost())

	fmt.Println(credential)
	ctx, cancelFunc := context.WithTimeout(context.Background(), keeper.GenerateTimeout)
	defer cancelFunc()
	token, err := keeper.GenerateToken(ctx, c.serverHost(), credential)
	if err != nil {
		log.Fatal(err)
	}

	c.apiPathPrefix = fmt.Sprintf("v2/users/%d", token.UserID)
	k.Token = token
	c.tokenProvider = k
}

func (c *Client) serverHost() string {
	u, err := url.Parse(c.endpoint)
	if err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
}
