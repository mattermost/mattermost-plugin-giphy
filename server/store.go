package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/plugin"
)

const (
	prefixToken = "token_"

	// Giphy ephemeral posts will stop working after this long
	expiryInSeconds = 60 * 30
)

type Store interface {
	StoreSecret(rootId, token string) error
	LoadSecret(rootId string) (string, error)
}

type store struct {
	api plugin.API
}

func NewStore(api plugin.API) Store {
	return &store{
		api: api,
	}
}

func hashkey(prefix, key string) string {
	h := md5.New()
	_, _ = h.Write([]byte(key))
	return fmt.Sprintf("%s%x", prefix, h.Sum(nil))
}

func (s store) setWithExpiry(key string, v interface{}, expiry int64) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	appErr := s.api.KVSetWithExpiry(key, data, expiry)
	if appErr != nil {
		return errors.New(appErr.Error())
	}
	return nil
}

func (s store) get(key string, v interface{}) error {
	data, appErr := s.api.KVGet(key)
	if appErr != nil {
		return errors.New(appErr.Error())
	}

	if data == nil {
		return nil
	}

	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}

	return nil
}

func (s store) StoreSecret(rootId, token string) error {
	return s.setWithExpiry(hashkey(prefixToken, rootId), token, expiryInSeconds)
}

func (s store) LoadSecret(rootId string) (string, error) {
	var token string
	err := s.get(hashkey(prefixToken, rootId), &token)
	if err != nil {
		return "", err
	}
	return token, nil
}
