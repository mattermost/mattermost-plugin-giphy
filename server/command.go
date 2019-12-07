package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
)

const Command = "giphy"

// OnActivate register the plugin command
func (p *Plugin) InitCommand() error {
	return p.API.RegisterCommand(&model.Command{
		Description:      "Pours Giphy's secret sauce",
		DisplayName:      "Giphy",
		AutoComplete:     true,
		AutoCompleteDesc: "Unleash the magic of Giphy",
		AutoCompleteHint: "[search string]",
		Trigger:          Command,
	})
}

func (p *Plugin) executeCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !strings.HasPrefix(args.Command, "/"+Command) {
		return respondCommandf("Invalid command: %s", args.Command)
	}
	q := strings.TrimSpace(args.Command[1+len(Command):])

	linkURL, embedURL, err := Query(p.getConfig(), q)
	if err != nil {
		return respondCommandf("Giphy API error: %v", err)
	}

	postActionContext := &PostActionContext{
		ChannelId: args.ChannelId,
		ParentId:  args.ParentId,
		RootId:    args.RootId,
		LinkURL:   linkURL,
		EmbedURL:  embedURL,
		Query:     q,
	}

	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Attachments:  []*model.SlackAttachment{NewSlackAttachment(p.API, postActionContext, true)},
	}, nil
}

func respondCommandf(format string, args ...interface{}) (*model.CommandResponse, *model.AppError) {
	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         fmt.Sprintf(format, args...),
	}, nil
}
