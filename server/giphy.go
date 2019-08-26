package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func Query(cf Config, s string) (linkURL, embedURL string, err error) {
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
