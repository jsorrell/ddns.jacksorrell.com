package dyndns2

import "github.com/jsorrell/ddns.jacksorrell.com/ddns_request_handler"
import "github.com/julienschmidt/httprouter"
import "net"
import "encoding/base64"
import "net/http"
import "strings"
import "errors"
import "bytes"

func HandleDDNSUpdateRequest(r *http.Request, ps httprouter.Params, f func(*ddns_request_handler.DDNSUpdateParameters)) error {
	var ddup ddns_request_handler.DDNSUpdateParameters

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Basic ") {
		return errors.New("Incorrect authorization format")
	}
	userPass, err := base64.StdEncoding.DecodeString(authHeader[6:])
	if err != nil {
		return err
	}
	splitUserPass := bytes.SplitN(userPass, []byte{':'}, 2)
	ddup.Username = string(splitUserPass[0])
	ddup.Password = string(splitUserPass[1])
	ddup.Hostname = r.URL.Query().Get("hostname")
	if ddup.Hostname == "" {
		return errors.New("No hostname given")
	}
	ddup.IP = net.ParseIP(r.URL.Query().Get("ip"))
	if ddup.IP == nil {
		ddup.IP = net.ParseIP(r.Header.Get("X-Real-IP"))
		if ddup.IP == nil {
			return errors.New("No X-Real-IP header set")
		}
	}

	f(&ddup)
	return nil
}
