package zdns

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
)

/**
{
 "resource_type": "gen_api_token",
  "attrs": {
    "encrypt": 0,
    "expire_in": 168,
    "grant_type": "password",
    "username": "gaoshiming",
    "password": "5yzd@oxC4mc7"
  }
}
*/
func NewLoginForm(username string, password string) interface{} {
	form := struct {
		Type  string `json:"resource_type"`
		Attrs struct {
			Encrypt   int    `json:"encrypt"`
			ExpireIn  int    `json:"expire_in"`
			GrantType string `json:"grant_type"`
			Username  string `json:"username"`
			Password  string `json:"password"`
		} `json:"attrs"`
	}{
		Type: "gen_api_token",
	}
	form.Attrs.Encrypt = 0
	form.Attrs.ExpireIn = 750
	form.Attrs.GrantType = "grant_type"
	form.Attrs.Username = username
	form.Attrs.Password = password
	return form
}

type ZdnsUser struct {
	Token    string    `json:"token"`
	IsAlias  bool      `json:"is_alias"`
	Username string    `json:"username"`
	ExpireIn int64     `json:"expire_in"`
	ExpireAt time.Time `json:"expire_at"`
}

func Login(user string, pass string) ZdnsUser {
	var err error
	data := NewLoginForm(user, pass)
	b, err := json.Marshal(data)
	exitIfError(err)
	r, _ := http.Post(api.GetAuthUrl(), "application/json", bytes.NewReader(b))
	resp, _ := ioutil.ReadAll(r.Body)
	u := ZdnsUser{}
	err = json.Unmarshal(resp, &u)
	exitIfError(err)
	t := time.Now()
	ts := t.Unix() + u.ExpireIn
	u.ExpireAt = time.Unix(ts, 0)
	u.Username = user

	return u
}
