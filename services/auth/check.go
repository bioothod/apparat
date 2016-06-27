package auth

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
)

func CheckCookieWeb(auth_url string, cookie *http.Cookie) (*AuthCookie, error) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", auth_url, nil)
	if err != nil {
		glog.Errorf("could not create new auth check request, url: %s, error: %v", auth_url, err)
		return nil, fmt.Errorf("could not create new auth check request, url: %s, error: %v", auth_url, err)
	}

	req.AddCookie(cookie)

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("could not request cookie check over http, url: %s, error: %v", auth_url, err)
		return nil, fmt.Errorf("could not request cookie check over http, url: %s, error: %v", auth_url, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Errorf("could not read request body, url: %s, error: %v", auth_url, err)
		return nil, fmt.Errorf("could not read request body, url: %s, error: %v", auth_url, err)
	}

	type AuthReply struct{
		Operation		string		`json:"operation"`
		Ac			AuthCookie	`json:"auth"`
	}

	var reply AuthReply

	err = json.Unmarshal(body, &reply)
	if err != nil {
		glog.Errorf("could not decode reply: '%s', error: %v", string(body), err)
		return nil, fmt.Errorf("could not decode reply: '%s', error: %v", string(body), err)
	}

	return &reply.Ac, nil
}
