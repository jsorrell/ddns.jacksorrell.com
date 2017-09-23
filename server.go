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
)

const configFileName string = "config.json"

type config struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Token string `json:"digitalocean_token"`
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
	usernameRegex := regexp.MustCompile(`^[[:alpha:]]+$`)
	//http://www.sussex.ac.uk/its/help/faq?faqid=839
	passwordRegex := regexp.MustCompile(`^[a-zA-Z0-9{}()\]\]#:;^,.?!|&_` + "`" + `~@$%/\\=+\-*"' ]{10,127}$`)
	tokenRegex := regexp.MustCompile(`^[[:xdigit:]]{64}$`)
	if (!usernameRegex.MatchString(conf.Username)) {
		return errors.New("invalid username")
	} else if (!passwordRegex.MatchString(conf.Password)) {
		return errors.New("invalid password: requirements here: http://www.sussex.ac.uk/its/help/faq?faqid=839")
	} else if (!tokenRegex.MatchString(conf.Token)) {
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
				makeBlankConfig()
				log.Fatal("No config file exists. Blank config file created.")
		}
		log.Fatal(err)
	}

	err = conf.validate()
	if (err != nil) {
		log.Fatal(err)
	}

	router := httprouter.New()
	router.POST("/update", getHandler(&conf))

	fmt.Println("\033[0;32mServer started\033[0m")
	log.Fatal(http.ListenAndServe(":9096", router))
}

func makeBlankConfig() {
	blankConfig := &config{
		Username: "",
		Password: "",
		Token: "",
	}

	blankConfig.write()
}

/***************************************************/
/*****************Update Handler********************/
/***************************************************/
func getHandler(conf *config) func(http.ResponseWriter, *http.Request, httprouter.Params)  {
	return func (w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		buf := make([]byte, r.ContentLength)
		io.ReadFull(r.Body, buf)
		type DDNSUpdateRequest struct {
			Username  string `json:"user"`
			Password string `json:"pass"`
			Domain string `json:"domain"`
			Ip string `json:"ip,omitempty"`
		}
		var ddnsUpdateRequest DDNSUpdateRequest
		err := json.Unmarshal(buf, &ddnsUpdateRequest)
		if err != nil {
			log.Println("ERROR: Error processing request: ", err)
			fmt.Fprint(w, "Error processing request\n")
			return
		} else if (ddnsUpdateRequest.Username != conf.Username) {
			log.Println("Incorrect username")
			fmt.Fprint(w, "Incorrect username\n")
			return
		} else if (ddnsUpdateRequest.Password != conf.Password) {
			log.Println("Incorrect password")
			fmt.Fprint(w, "Incorrect password\n")
			return
		}

		digitaloceanClient := digitalocean.GetClient(conf.Token)
		err = digitaloceanClient.UpdateRecord(ddnsUpdateRequest.Domain, ddnsUpdateRequest.Ip)
		if (err != nil) {
			log.Println("ERROR: Error updating dns: " + err.Error())
			fmt.Fprint(w, "ERROR: Error updating dns: " + err.Error())
			return
		}

		fmt.Fprint(w, "Successfully updated")
	}
}
