package main

import (
	"fmt"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

func AddSlackAttachment(api plugin.API, post *model.Post, c *PostActionContext, interactive bool) {
	if post.Props == nil {
		post.Props = map[string]interface{}{}
	}
	post.Props["attachments"] = []*model.SlackAttachment{NewSlackAttachment(api, c, interactive)}
}

func NewSlackAttachment(api plugin.API, c *PostActionContext, interactive bool) *model.SlackAttachment {
	sa := &model.SlackAttachment{
		Title:     c.Query,
		TitleLink: c.LinkURL,
		Text:      fmt.Sprintf("URL: %s\nPosted using [/giphy](https://www.giphy.com). ", c.LinkURL),
		ImageURL:  c.EmbedURL,
	}

	if interactive {
		AddPostActions(api, sa, c)
	}

	return sa
}
