package zdns

import (
	"fmt"
	"net/url"
	"github.com/gsmlg-dev/gsmlg-cli/gsmlg/errorhandler"
)

var exitIfError = errorhandler.CreateExitIfError("ZDNS")
var host string = "https://cloud.zdns.cn"

var api *ApiService = NewApi()

type ApiService struct {
	baseUrl string
	token string
}

func NewApi() *ApiService {
	api := &ApiService {
		baseUrl: host,
	}
	return api
}

func SetToken(t string) {
	api.SetToken(t)
}

func (api *ApiService) SetHost(h string) {
	api.baseUrl = h
}

func (api *ApiService) SetToken(t string) {
	api.token = t
}

func (api *ApiService) GetAuthUrl() string {
	s := fmt.Sprintf("%s/%s", api.baseUrl, "auth_cmd")
	return s
}

func (api *ApiService) GetRRManagerUrl() *url.URL {
	u, err := url.Parse(api.baseUrl)
	exitIfError(err)
	u.Path = "/rrmanager"
	q := u.Query()
	q.Set("zdnsuser", api.token)
	u.RawQuery = q.Encode()
	return u
}
