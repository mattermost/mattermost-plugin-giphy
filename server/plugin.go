package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/blang/semver"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
)

// TODO: require 5.10?
const (
	minimumServerVersion = "5.8.0"
)

type Plugin struct {
	plugin.MattermostPlugin
	router *mux.Router

	cLock sync.RWMutex
	cf    *Config
}

type Config struct {
	Trigger string
	Rating  string
	APIKey  string
}

// OnActivate register the plugin command
func (p *Plugin) OnActivate() error {
	// Check server version
	serverVersion, err := semver.Parse(p.API.GetServerVersion())
	if err != nil {
		return errors.Wrap(err, "failed to parse server version")
	}

	r := semver.MustParseRange(">=" + minimumServerVersion)
	if !r(serverVersion) {
		return fmt.Errorf("this plugin requires Mattermost v%s or later", minimumServerVersion)
	}

	router := mux.NewRouter()
	s := router.PathPrefix("/api/v1").Subrouter()
	s.HandleFunc("/send", p.handleSend).Methods("POST")
	s.HandleFunc("/shuffle", p.handleShuffle).Methods("POST")
	s.HandleFunc("/cancel", p.handleCancel).Methods("POST")
	p.router = router

	return nil
}

func (p *Plugin) OnConfigurationChange() error {
	oldcf := p.getConfig()
	newcf := Config{}

	err := p.API.LoadPluginConfiguration(&newcf)
	if err != nil {
		return errors.Wrap(err, "Failed to load plugin configuration")
	}

	if newcf.Trigger == "" {
		return errors.New("Empty trigger not allowed")
	}
	if oldcf.Trigger != "" {
		err := p.API.UnregisterCommand("", oldcf.Trigger)
		if err != nil {
			return errors.Wrap(err, "failed to unregister old command")
		}
	}
	err = p.API.RegisterCommand(&model.Command{
		Description:      "Pours Giphy's secret sauce",
		DisplayName:      "Giphy",
		AutoComplete:     true,
		AutoCompleteDesc: "Unleash the magic of Giphy",
		AutoCompleteHint: "[search string]",
		Trigger:          newcf.Trigger,
	})
	if err != nil {
		return errors.Wrap(err, "failed to register new command")
	}

	p.cLock.Lock()
	p.cf = &newcf
	p.cLock.Unlock()

	return nil
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	cf := p.getConfig()

	if !strings.HasPrefix(args.Command, "/"+cf.Trigger) {
		return getErrorCommandResponse("Invalid command: " + args.Command), nil
	}
	s := strings.TrimSpace(args.Command[len(cf.Trigger)+1:])

	linkURL, embedURL, err := query(*cf, s)
	if err != nil {
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         "Giphy API error: " + err.Error(),
		}, nil
	}

	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Attachments:  []*model.SlackAttachment{p.newGiphyAttachment("", s, linkURL, embedURL, true)},
	}, nil
}

func (p *Plugin) handleSend(w http.ResponseWriter, r *http.Request) {
	request := model.PostActionIntegrationRequestFromJson(r.Body)
	if request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	channelID, queryString, linkURL, embedURL := decodeContext(request.Context)
	if request.ChannelId != "" {
		channelID = request.ChannelId
	}

	// Remove the old ephemeral message
	post := model.Post{
		Id:     request.PostId,
		Type:   model.POST_EPHEMERAL,
		UserId: request.UserId,
	}
	p.API.DeleteEphemeralPost(request.UserId, &post)

	// Create the in-channel post
	post = model.Post{
		UserId:    request.UserId,
		ChannelId: channelID,
		Type:      model.POST_DEFAULT,
		Props: map[string]interface{}{
			"attachments": []*model.SlackAttachment{p.newGiphyAttachment(channelID, queryString, linkURL, embedURL, false)},
		},
	}
	// post.AddProp("from_webhook", "true")

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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := &model.PostActionIntegrationResponse{}
	_, _ = w.Write(response.ToJson())
}

func (p *Plugin) handleShuffle(w http.ResponseWriter, r *http.Request) {
	cf := p.getConfig()

	request := model.PostActionIntegrationRequestFromJson(r.Body)
	if request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	channelID, queryString, _, _ := decodeContext(request.Context)
	if request.ChannelId != "" {
		channelID = request.ChannelId
	}

	linkURL, embedURL, err := query(*cf, queryString)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := &model.PostActionIntegrationResponse{
			EphemeralText: "Giphy API error: " + err.Error(),
		}
		_, _ = w.Write(response.ToJson())
		return
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
	// post.AddProp("from_webhook", "true")

	p.API.UpdateEphemeralPost(request.UserId, &post)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := &model.PostActionIntegrationResponse{}
	_, _ = w.Write(response.ToJson())
}

func (p *Plugin) handleCancel(w http.ResponseWriter, r *http.Request) {
	request := model.PostActionIntegrationRequestFromJson(r.Body)
	if request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, queryString, _, _ := decodeContext(request.Context)

	post := model.Post{
		Id:       request.PostId,
		Type:     model.POST_EPHEMERAL,
		Message:  `Cancelled giphy: "` + queryString + `"`,
		UserId:   request.UserId,
		CreateAt: model.GetMillis(),
		UpdateAt: model.GetMillis(),
	}
	// post.AddProp("from_webhook", "true")

	p.API.UpdateEphemeralPost(request.UserId, &post)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := &model.PostActionIntegrationResponse{}
	_, _ = w.Write(response.ToJson())
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

func (p *Plugin) getConfig() *Config {
	p.cLock.RLock()
	defer p.cLock.RUnlock()

	if p.cf == nil {
		return &Config{}
	}
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
		Text:      "Posted using [/giphy](https://www.giphy.com)",
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

func decodeContext(context map[string]interface{}) (channelID, queryString, linkURL, embedURL string) {
	if context == nil {
		return "", "", "", ""
	}
	channelID = context["ChannelID"].(string)
	queryString = context["QueryString"].(string)
	linkURL = context["LinkURL"].(string)
	embedURL = context["EmbedURL"].(string)

	return channelID, queryString, linkURL, embedURL
}

func getErrorCommandResponse(text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         text,
	}
}
