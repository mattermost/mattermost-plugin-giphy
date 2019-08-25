package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
)

const Command = "giphy"

type Plugin struct {
	plugin.MattermostPlugin
	router *mux.Router

	cLock sync.RWMutex
	cf    Config
}

type Config struct {
	Rating string
	APIKey string
}

// OnActivate register the plugin command
func (p *Plugin) OnActivate() error {
	router := mux.NewRouter()
	s := router.PathPrefix("/api/v1").Subrouter()
	s.HandleFunc("/send", p.handleSend).Methods("POST")
	s.HandleFunc("/shuffle", p.handleShuffle).Methods("POST")
	s.HandleFunc("/cancel", p.handleCancel).Methods("POST")
	p.router = router

	err := p.API.RegisterCommand(&model.Command{
		Description:      "Pours Giphy's secret sauce",
		DisplayName:      "Giphy",
		AutoComplete:     true,
		AutoCompleteDesc: "Unleash the magic of Giphy",
		AutoCompleteHint: "[search string]",
		Trigger:          Command,
	})
	if err != nil {
		return errors.WithMessagef(err, "failed to register /%s command", Command)
	}
	return nil
}

func (p *Plugin) OnConfigurationChange() error {
	cf := Config{}
	err := p.API.LoadPluginConfiguration(&cf)
	if err != nil {
		return errors.WithMessage(err, "Failed to load plugin configuration")
	}

	p.cLock.Lock()
	p.cf = cf
	p.cLock.Unlock()
	return nil
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !strings.HasPrefix(args.Command, "/"+Command) {
		return respondCommandf("Invalid command: %s", args.Command)
	}
	s := strings.TrimSpace(args.Command[1+len(Command):])

	linkURL, embedURL, err := query(p.getConfig(), s)
	if err != nil {
		return respondCommandf("Giphy API error: %v", err)
	}

	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Attachments:  []*model.SlackAttachment{p.newGiphyAttachment("", s, linkURL, embedURL, true)},
	}, nil
}

func (p *Plugin) handleSend(w http.ResponseWriter, r *http.Request) {
	request, channelID, queryString, linkURL, embedURL := decodePostActionRequest(r)
	if request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Remove the old ephemeral message
	p.API.DeleteEphemeralPost(request.UserId, request.PostId)

	// Create the in-channel post
	post := model.Post{
		UserId:    request.UserId,
		ChannelId: channelID,
		Type:      model.POST_DEFAULT,
		Props: map[string]interface{}{
			"attachments": []*model.SlackAttachment{p.newGiphyAttachment(channelID, queryString, linkURL, embedURL, false)},
		},
	}

	_, err := p.API.CreatePost(&post)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := &model.PostActionIntegrationResponse{
			EphemeralText: "Error: " + err.Error(),
		}
		_, _ = w.Write(response.ToJson())
		return
	}

	respondPostActionf(w, "")
}

func (p *Plugin) handleShuffle(w http.ResponseWriter, r *http.Request) {
	request, channelID, queryString, _, prevEmbedURL := decodePostActionRequest(r)
	if request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tryShuffle := func() (done bool) {
		linkURL, embedURL, err := query(p.getConfig(), queryString)
		if err != nil {
			respondPostActionf(w, "Giphy API error: %v", err)
			return true
		}
		if embedURL == prevEmbedURL {
			return false
		}

		post := model.Post{
			Id:        request.PostId,
			Type:      model.POST_EPHEMERAL,
			UserId:    request.UserId,
			ChannelId: channelID,
			CreateAt:  model.GetMillis(),
			UpdateAt:  model.GetMillis(),
			Props: map[string]interface{}{
				"attachments": []*model.SlackAttachment{p.newGiphyAttachment(channelID, queryString, linkURL, embedURL, true)},
			},
		}
		p.API.UpdateEphemeralPost(request.UserId, &post)
		return true
	}

	for n := 0; n < 3; n++ {
		if tryShuffle() {
			break
		}
	}

	respondPostActionf(w, "")
}

func (p *Plugin) handleCancel(w http.ResponseWriter, r *http.Request) {
	request, channelId, queryString, _, _ := decodePostActionRequest(r)
	if request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	post := model.Post{
		Id:        request.PostId,
		ChannelId: channelId,
		Type:      model.POST_EPHEMERAL,
		Message:   `Cancelled giphy: "` + queryString + `"`,
		UserId:    request.UserId,
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
	}

	p.API.UpdateEphemeralPost(request.UserId, &post)

	respondPostActionf(w, "")
}

func query(cf Config, s string) (linkURL, embedURL string, err error) {
	q := url.Values{}
	q.Set("api_key", cf.APIKey)
	q.Set("s", s)
	q.Set("weirdness", "10")
	q.Set("rating", cf.Rating)
	urlstr := fmt.Sprintf("https://api.giphy.com/v1/gifs/translate?%s", q.Encode())

	resp, err := http.Get(urlstr)
	if err != nil {
		return "", "", err
	}
	if resp.Body == nil {
		return "", "", fmt.Errorf("empty response, status:%d", resp.StatusCode)
	}
	defer resp.Body.Close()

	g := map[string]interface{}{}
	err = json.NewDecoder(resp.Body).Decode(&g)
	if err != nil {
		return "", "", err
	}
	gdata, _ := g["data"].(map[string]interface{})
	if gdata == nil {
		meta, _ := g["meta"].(map[string]interface{})
		if meta != nil {
			return "", "", fmt.Errorf("Giphy API error:%v", meta["msg"])
		} else {
			return "", "", fmt.Errorf("Giphy API error: empty response, status:%d", resp.StatusCode)
		}
	}

	return gdata["url"].(string), fmt.Sprintf("https://media.giphy.com/media/%s/giphy.gif", gdata["id"]), nil
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

func (p *Plugin) getConfig() Config {
	p.cLock.RLock()
	defer p.cLock.RUnlock()

	return p.cf
}

func (p *Plugin) actionURL(action string) string {
	return fmt.Sprintf("%s/plugins/%s/api/v1/%s", *p.API.GetConfig().ServiceSettings.SiteURL,
		manifest.Id, action)
}

func (p *Plugin) newGiphyAttachment(channelId, queryString, linkURL, embedURL string, interactive bool) *model.SlackAttachment {
	a := model.SlackAttachment{
		Title:     queryString,
		TitleLink: linkURL,
		Text:      fmt.Sprintf("URL: %s\nPosted using [/giphy](https://www.giphy.com). ", linkURL),
		ImageURL:  embedURL,
	}

	if !interactive {
		return &a
	}

	context := map[string]interface{}{
		"ChannelID":   channelId,
		"QueryString": queryString,
		"EmbedURL":    embedURL,
		"LinkURL":     linkURL,
	}

	a.Actions = []*model.PostAction{
		{
			Id:   model.NewId(),
			Name: "Send",
			Type: model.POST_ACTION_TYPE_BUTTON,
			Integration: &model.PostActionIntegration{
				URL:     p.actionURL("send"),
				Context: context,
			},
		},
		{
			Id:   model.NewId(),
			Name: "Shuffle",
			Type: model.POST_ACTION_TYPE_BUTTON,
			Integration: &model.PostActionIntegration{
				URL:     p.actionURL("shuffle"),
				Context: context,
			},
		},
		{
			Id:   model.NewId(),
			Name: "Cancel",
			Type: model.POST_ACTION_TYPE_BUTTON,
			Integration: &model.PostActionIntegration{
				URL:     p.actionURL("cancel"),
				Context: context,
			},
		},
	}

	return &a
}

func decodePostActionRequest(r *http.Request) (request *model.PostActionIntegrationRequest, channelID, queryString, linkURL, embedURL string) {
	request = model.PostActionIntegrationRequestFromJson(r.Body)
	if request == nil || request.Context == nil {
		return nil, "", "", "", ""
	}

	context := request.Context
	channelID = context["ChannelID"].(string)
	queryString = context["QueryString"].(string)
	linkURL = context["LinkURL"].(string)
	embedURL = context["EmbedURL"].(string)

	if request.ChannelId != "" {
		channelID = request.ChannelId
	}

	return request, channelID, queryString, linkURL, embedURL
}

func respondCommandf(format string, args ...interface{}) (*model.CommandResponse, *model.AppError) {
	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         fmt.Sprintf(format, args...),
	}, nil
}

func respondPostActionf(w http.ResponseWriter, format string, args ...interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := &model.PostActionIntegrationResponse{
		EphemeralText: fmt.Sprintf(format, args...),
	}
	_, _ = w.Write(response.ToJson())
}
