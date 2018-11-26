package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
)

const (
	trigger = "giphy"
	command = "/giphy"
	apiKey  = "0xtKP4cGBIFI6tnFemSZ9TxTvQdzys7i"
)

type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration
}

// OnActivate register the plugin command
func (p *Plugin) OnActivate() error {
	return p.API.RegisterCommand(&model.Command{
		Trigger:          "giphy",
		Description:      "Pours Giphy's secret sauce",
		DisplayName:      "Giphy",
		AutoComplete:     true,
		AutoCompleteDesc: "Unleashes the magic of Giphy",
		AutoCompleteHint: "[search string]",
	})
}

// ExecuteCommand executes a command that has been previously registered via the RegisterCommand
// API.
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !strings.HasPrefix(args.Command, command) {
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         fmt.Sprintf("Unknown command: " + args.Command),
		}, nil
	}

	s := strings.TrimSpace(args.Command[len(command):])

	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("s", s)
	q.Set("weirdness", "10")
	q.Set("rating", "g")
	urlstr := fmt.Sprintf("https://api.giphy.com/v1/gifs/translate?%s", q.Encode())

	resp, err := http.Get(urlstr)
	if err != nil {
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         "Giphy error: " + err.Error(),
		}, nil
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	g := map[string]interface{}{}
	err = json.NewDecoder(resp.Body).Decode(&g)
	if err != nil {
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         "Giphy error: " + err.Error(),
		}, nil
	}
	gdata := g["data"].(map[string]interface{})

	linkURL := gdata["url"].(string)
	embedURL := fmt.Sprintf("https://media.giphy.com/media/%s/giphy.gif", gdata["id"])
	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_IN_CHANNEL,
		Text:         fmt.Sprintf("#### [%s](%s)\n*(Posted using [/giphy](https://www.giphy.com/))*\n![](%s)", s, linkURL, embedURL),
	}, nil
}
