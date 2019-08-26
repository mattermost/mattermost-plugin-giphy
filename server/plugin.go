package main

import (
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
)

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
	p.router = mux.NewRouter()
	p.InitPostActionRoutes()

	err := p.InitCommand()
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
	return p.executeCommand(args)
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

func (p *Plugin) getConfig() Config {
	p.cLock.RLock()
	defer p.cLock.RUnlock()

	return p.cf
}
