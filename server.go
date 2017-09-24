package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jsorrell/ddns.jacksorrell.com/ddns_request_handler"
	"github.com/jsorrell/ddns.jacksorrell.com/ddns_request_handler/dyndns2"
	"github.com/jsorrell/ddns.jacksorrell.com/digitalocean"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
)

const configFileName string = "config.json"

type config struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	DigitalOcean_Token string `json:"digitalocean_token"`
}

func (conf *config) write() error {
	buf, err := json.MarshalIndent(conf, "", "\t")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(configFileName, buf, 0600)
}

func readConfig(conf *config) error {
	buf, err := ioutil.ReadFile(configFileName)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, conf)
}

func (conf *config) validate() error {
	usernameRegex := regexp.MustCompile(`^[[:alpha:]]+$`)
	//http://www.sussex.ac.uk/its/help/faq?faqid=839
	passwordRegex := regexp.MustCompile(`^[a-zA-Z0-9{}()\]\]#:;^,.?!|&_` + "`" + `~@$%/\\=+\-*"' ]{10,127}$`)
	tokenRegex := regexp.MustCompile(`^[[:xdigit:]]{64}$`)
	if !usernameRegex.MatchString(conf.Username) {
		return errors.New("invalid username")
	} else if !passwordRegex.MatchString(conf.Password) {
		return errors.New("invalid password: requirements here: http://www.sussex.ac.uk/its/help/faq?faqid=839")
	} else if !tokenRegex.MatchString(conf.DigitalOcean_Token) {
		return errors.New("invalid digitalocean_token")
	} else {
		return nil
	}
}

func (conf *config) getUserPass64Enc() string {
	userPass := []byte(conf.Username + ":" + conf.Password)
	return base64.StdEncoding.EncodeToString(userPass)
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
	if err != nil {
		switch err.Error() {
		case "open " + configFileName + ": no such file or directory":
			if makeBlankConfig() != nil {
				log.Fatal("Error creating blank config: ", err)
			}
			log.Fatal("No config file exists. Blank config file created.")
		}
		log.Fatal(err)
	}

	err = conf.validate()
	if err != nil {
		log.Fatal(err)
	}

	router := httprouter.New()
	router.GET("/nic/update", getUpdateHandler(dyndns2.HandleDDNSUpdateRequest, &conf))

	fmt.Println("\033[0;32mServer started\033[0m")
	log.Fatal(http.ListenAndServe(":9096", router))
}

func makeBlankConfig() error {
	blankConfig := &config{
		Username:           "",
		Password:           "",
		DigitalOcean_Token: "",
	}

	return blankConfig.write()
}

/***************************************************/
/*****************Update Handler********************/
/***************************************************/
func getUpdateHandler(ddnsHandler ddns_request_handler.DDNSRequestHandler, conf *config) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		err := ddnsHandler(r, ps, func(ddup *ddns_request_handler.DDNSUpdateParameters) {
			if ddup.Username != conf.Username || ddup.Password != conf.Password {
				log.Println("Incorrect login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			digitaloceanClient := digitalocean.GetClient(conf.DigitalOcean_Token)
			status, err := digitaloceanClient.UpdateRecord(ddup.Hostname, ddup.IP)
			switch status {
			case digitalocean.OK:
				log.Println("Successfully updated ", ddup.Hostname, " to ", ddup.IP)
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
				log.Println("Domain ", ddup.Hostname, " not registered")
				w.WriteHeader(http.StatusNotFound)
				return
			case digitalocean.INTERNAL_SERVER_ERROR:
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		})

		if err != nil {
			log.Println("ERROR: Error processing request: ", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
}
