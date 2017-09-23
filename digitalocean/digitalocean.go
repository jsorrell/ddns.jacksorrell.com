package digitalocean

import "github.com/digitalocean/godo"
import "golang.org/x/oauth2"
import "regexp"
import "os"
import "context"
import "errors"
import "strconv"
import "log"

type DDNSClient struct {
	*godo.Client
}

type TokenSource struct {
    AccessToken string
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
    token := &oauth2.Token{
        AccessToken: t.AccessToken,
    }
    return token, nil
}

func GetClient(token string) *DDNSClient {
	var tokenSource = &TokenSource{
	    AccessToken: token,
	}
	oauthClient := oauth2.NewClient(context.TODO(), tokenSource)
	return &DDNSClient{godo.NewClient(oauthClient)}
}

func (client DDNSClient) UpdateRecord(subdomain string, ip string) error {
	if (len(subdomain) > 64) {
		return errors.New("subdomain length of " + strconv.Itoa(len(subdomain)) + " is longer than 64 and invalid")
	}

	re := regexp.MustCompile(`^((?:[a-zA-Z\-]+\.)+)([a-zA-Z\-]+\.[a-z]+)$`)
	matches := re.FindStringSubmatch(subdomain)

	if (matches == nil) {
		return errors.New("Subdomain is not a valid format")
	}

	domain := matches[2]
	//Remove trailing dot
	domainPrefix := matches[1][:len(matches[1])-1]

	domainsService := client.Domains
	//FIXME: won't work with more than 1000 records
	paginationOptions := &godo.ListOptions{ PerPage: 1000 }
	records, _, err := domainsService.Records(context.TODO(), domain, paginationOptions)

	if (err != nil) {
		return err
	}

	var domainRecord *godo.DomainRecord = nil
	for _, v := range records {
		if (v.Type == "A" && v.Name == domainPrefix) {
			domainRecord = &v
		}
	}

	if (domainRecord == nil) {
		return errors.New("Subdomain not found")
	}

	if (domainRecord.Data == ip) {
		log.Println("Record already up to date")
		return nil
	}

	drer := &godo.DomainRecordEditRequest{ Data: ip }
	_, _, err = domainsService.EditRecord(context.TODO(), domain, domainRecord.ID, drer)
	if (err == nil) {
		log.Println(os.Stderr, "DNS successfully updated")
		return nil
	} else {
		return err
	}
}
