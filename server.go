package main

import (
	"fmt"
	"github.com/julienschmidt/httprouter"
	"github.com/jsorrell/ddns.jacksorrell.com/digitalocean"
	"log"
	"net/http"
	"os"
	"path"
	"io"
	"io/ioutil"
	"encoding/json"
	"regexp"
	"errors"
	"encoding/hex"
	"crypto/rand"
	"strings"
	"net"
)

const configFileName string = "config.json"

type config struct {
	Token string `json:"token"`
	DigitalOcean_Token string `json:"digitalocean_token"`
}

func (conf *config) write() error {
	buf, err := json.MarshalIndent(conf,"","\t")
	if (err != nil) {
		return err
	}

	return ioutil.WriteFile(configFileName, buf, 0600)
}

func readConfig(conf *config) error {
	buf, err := ioutil.ReadFile(configFileName)
	if (err != nil) {
		return err
	}
	return json.Unmarshal(buf, conf)
}

func (conf *config) validate() error {
	tokenRegex := regexp.MustCompile(`^[[:xdigit:]]{64}$`)
	if (!tokenRegex.MatchString(conf.Token)) {
		return errors.New("invalid token")
	} else if (!tokenRegex.MatchString(conf.DigitalOcean_Token)) {
		return errors.New("invalid digitalocean_token")
	} else {
		return nil
	}
}

/****************************************************/
/*****************Server Setup***********************/
/****************************************************/
func main() {
	fmt.Println("Starting server...")
	wd := path.Join(os.Getenv("GOPATH"), "src/github.com/jsorrell/ddns.jacksorrell.com")
	os.Chdir(wd)

	//Open config file
	var conf config
	err := readConfig(&conf)
	if (err != nil) {
		switch err.Error() {
			case "open " + configFileName + ": no such file or directory":
				if (makeBlankConfig() != nil) {
					log.Fatal("Error creating blank config: ", err)
				}
				log.Fatal("No config file exists. Blank config file created.")
		}
		log.Fatal(err)
	}

	err = conf.validate()
	if (err != nil) {
		log.Fatal(err)
	}

	router := httprouter.New()
	router.PUT("/:domain", getUpdateHandler(&conf))

	fmt.Println("\033[0;32mServer started\033[0m")
	log.Fatal(http.ListenAndServe(":9096", router))
}

func makeBlankConfig() error {
	bs := make([]byte, 32)
	n, err := rand.Read(bs)
	if (n != 32 || err != nil) {
		return err
	}
	blankConfig := &config{
		Token: hex.EncodeToString(bs),
		DigitalOcean_Token: "",
	}

	return blankConfig.write()
}

/***************************************************/
/*****************Update Handler********************/
/***************************************************/
func getUpdateHandler(conf *config) func(http.ResponseWriter, *http.Request, httprouter.Params)  {
	return func (w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		authHeader := r.Header.Get("Authorization")
		if (!strings.HasPrefix(authHeader, "Bearer ")) {
			log.Println("Incorrect authorization format")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		requestToken := authHeader[7:]

		if requestToken != conf.Token {
			log.Println("Incorrect auth token")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		type DDNSUpdateRequest struct {
			Ip string `json:"ip,omitempty"`
		}
		var ddnsUpdateRequest DDNSUpdateRequest
		var err error
		if (r.ContentLength > 0) {
			buf := make([]byte, r.ContentLength)
			io.ReadFull(r.Body, buf)
			err = json.Unmarshal(buf, &ddnsUpdateRequest)
			if err != nil {
				log.Println("ERROR: Error processing request: ", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		parsedIP := net.ParseIP(ddnsUpdateRequest.Ip)
		requestIP := r.Header.Get("X-Real-IP")
		if requestIP == "" {
			log.Println("ERROR: Configuration error: No X-Real-IP header set")
			//Don't return here to allow given to be used
			if (parsedIP == nil) {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}

		ip := parsedIP
		if ip == nil {
			ip = net.ParseIP(requestIP)
		}

		digitaloceanClient := digitalocean.GetClient(conf.DigitalOcean_Token)
		status, err := digitaloceanClient.UpdateRecord(ps.ByName("domain"), ip)
		switch status {
		case digitalocean.OK:
			log.Println("Successfully updated ", ps.ByName("domain"), " to ", ip)
			w.WriteHeader(http.StatusOK)
			return
		case digitalocean.NOT_MODIFIED:
			log.Println("Record already up to date")
			w.WriteHeader(http.StatusNotModified)
			return
		case digitalocean.BAD_REQUEST:
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		case digitalocean.NOT_FOUND:
			log.Println("Domain ", ps.ByName("domain"), " not registered")
			w.WriteHeader(http.StatusNotFound)
			return
		case digitalocean.INTERNAL_SERVER_ERROR:
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
