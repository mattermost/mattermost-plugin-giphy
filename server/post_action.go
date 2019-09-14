package main

import (
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
)

type PostActionContext struct {
	ChannelId string
	RootId    string
	ParentId  string
	Query     string
	EmbedURL  string
	LinkURL   string
}

func (p *Plugin) InitPostActionRoutes() {
	s := p.router.PathPrefix("/api/v1").Subrouter()
	s.HandleFunc("/send", p.handleSend).Methods("POST")
	s.HandleFunc("/shuffle", p.handleShuffle).Methods("POST")
	s.HandleFunc("/cancel", p.handleCancel).Methods("POST")
}

func decodePostActionRequest(r *http.Request) (*model.PostActionIntegrationRequest, *PostActionContext) {
	mattermostUserId := r.Header.Get("Mattermost-User-Id")
	if mattermostUserId == "" {
		return nil, nil
	}

	request := model.PostActionIntegrationRequestFromJson(r.Body)
	if request == nil || request.Context == nil {
		return nil, nil
	}

	context := request.Context
	c := &PostActionContext{
		ChannelId: context["ChannelId"].(string),
		RootId:    context["RootId"].(string),
		ParentId:  context["ParentId"].(string),
		Query:     context["Query"].(string),
		EmbedURL:  context["EmbedURL"].(string),
		LinkURL:   context["LinkURL"].(string),
	}

	if request.ChannelId != "" {
		c.ChannelId = request.ChannelId
	}

	return request, c
}

func AddPostActions(api plugin.API, sa *model.SlackAttachment, c *PostActionContext) {
	actionURL := func(action string) string {
		return fmt.Sprintf("%s/plugins/%s/api/v1/%s", *api.GetConfig().ServiceSettings.SiteURL,
			manifest.Id, action)
	}

	context := map[string]interface{}{
		"ChannelId": c.ChannelId,
		"RootId":    c.RootId,
		"ParentId":  c.ParentId,
		"Query":     c.Query,
		"EmbedURL":  c.EmbedURL,
		"LinkURL":   c.LinkURL,
	}

	sa.Actions = []*model.PostAction{
		{
			Id:   model.NewId(),
			Name: "Send",
			Type: model.POST_ACTION_TYPE_BUTTON,
			Integration: &model.PostActionIntegration{
				URL:     actionURL("send"),
				Context: context,
			},
		},
		{
			Id:   model.NewId(),
			Name: "Shuffle",
			Type: model.POST_ACTION_TYPE_BUTTON,
			Integration: &model.PostActionIntegration{
				URL:     actionURL("shuffle"),
				Context: context,
			},
		},
		{
			Id:   model.NewId(),
			Name: "Cancel",
			Type: model.POST_ACTION_TYPE_BUTTON,
			Integration: &model.PostActionIntegration{
				URL:     actionURL("cancel"),
				Context: context,
			},
		},
	}
}

func (p *Plugin) handleSend(w http.ResponseWriter, r *http.Request) {
	request, c := decodePostActionRequest(r)
	if request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Remove the old ephemeral message
	p.API.DeleteEphemeralPost(request.UserId, request.PostId)

	// Create the in-channel post
	post := &model.Post{
		UserId:    r.Header.Get("Mattermost-User-Id"),
		ChannelId: c.ChannelId,
		RootId:    c.RootId,
		ParentId:  c.ParentId,
		Type:      model.POST_DEFAULT,
	}
	AddSlackAttachment(p.API, post, c, false)

	_, appErr := p.API.CreatePost(post)
	if appErr != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := &model.PostActionIntegrationResponse{
			EphemeralText: "Error: " + appErr.Error(),
		}
		_, _ = w.Write(response.ToJson())
		return
	}

	respondPostActionf(w, "")
}

func (p *Plugin) handleShuffle(w http.ResponseWriter, r *http.Request) {
	request, c := decodePostActionRequest(r)
	if request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tryShuffle := func() (done bool) {
		linkURL, embedURL, err := Query(p.getConfig(), c.Query)
		if err != nil {
			respondPostActionf(w, "Giphy API error: %v", err)
			return true
		}
		if embedURL == c.EmbedURL {
			return false
		}

		c.LinkURL = linkURL
		c.EmbedURL = embedURL

		post := &model.Post{
			Id:        request.PostId,
			Type:      model.POST_EPHEMERAL,
			UserId:    request.UserId,
			ChannelId: c.ChannelId,
			RootId:    c.RootId,
			ParentId:  c.ParentId,
			CreateAt:  model.GetMillis(),
			UpdateAt:  model.GetMillis(),
		}
		AddSlackAttachment(p.API, post, c, true)

		p.API.UpdateEphemeralPost(request.UserId, post)
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
	request, c := decodePostActionRequest(r)
	if request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	post := &model.Post{
		Id:        request.PostId,
		ChannelId: c.ChannelId,
		RootId:    c.RootId,
		ParentId:  c.ParentId,
		Type:      model.POST_EPHEMERAL,
		Message:   `Cancelled giphy: "` + c.Query + `"`,
		UserId:    request.UserId,
		CreateAt:  model.GetMillis(),
		UpdateAt:  model.GetMillis(),
	}

	p.API.UpdateEphemeralPost(request.UserId, post)

	respondPostActionf(w, "")
}

func respondPostActionf(w http.ResponseWriter, format string, args ...interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := &model.PostActionIntegrationResponse{
		EphemeralText: fmt.Sprintf(format, args...),
	}
	_, _ = w.Write(response.ToJson())
}
