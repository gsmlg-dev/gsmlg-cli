package zdns

import (
	"net/http"
	"io"
	"fmt"
)

func GetZone() {
	u := api.GetRRManagerUrl()
	q := u.Query()
	q.Set("resource_type", "zone")
	u.RawQuery = q.Encode()
	s := u.String()
	resp, err := http.Get(s)
	exitIfError(err)
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	exitIfError(err)

	fmt.Printf("%s", data)
}
