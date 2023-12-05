package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	twitterBaseURL = "https://vxtwitter.com"
)

type discordPost struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

func postDiscord(username, name, tweet, webhook string) error {
	var text string
	if strings.HasPrefix(tweet, fmt.Sprintf("/%s/", username)) {
		text = "Tweeted"
	} else {
		text = "Retweeted"
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(&discordPost{
		Username: name,
		Content: fmt.Sprintf(
			"[%s](%s)",
			text,
			twitterBaseURL+tweet,
		),
	}); err != nil {
		return err
	}

	resp, err := http.Post(webhook, "application/json", &buf)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
	default:
		return fmt.Errorf("webhook server returned error: %s", resp.Status)
	}
	return nil
}
