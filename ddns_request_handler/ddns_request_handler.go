package ddns_request_handler

import "github.com/julienschmidt/httprouter"
import "net"
import "net/http"

type DDNSUpdateParameters struct {
	Username string
	Password string
	Hostname string
	IP       net.IP
}

type DDNSRequestHandler func(*http.Request, httprouter.Params, func(*DDNSUpdateParameters)) error
