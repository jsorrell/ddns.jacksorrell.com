package digitalocean

import "github.com/digitalocean/godo"
import "golang.org/x/oauth2"
import "regexp"
import "context"
import "errors"
import "strconv"
import "net"

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

type Status int

const (
	OK Status = iota
	NOT_MODIFIED
	BAD_REQUEST
	NOT_FOUND
	INTERNAL_SERVER_ERROR
)

func (client DDNSClient) UpdateRecord(subdomain string, ip net.IP) (Status, error) {
	if len(subdomain) > 64 {
		return BAD_REQUEST, errors.New("Subdomain length of " + strconv.Itoa(len(subdomain)) + " is longer than 64 and invalid")
	}

	re := regexp.MustCompile(`^((?:[a-zA-Z\-]+\.)+)([a-zA-Z\-]+\.[a-z]+)$`)
	matches := re.FindStringSubmatch(subdomain)

	if matches == nil {
		return BAD_REQUEST, errors.New("Subdomain is not a valid format")
	}

	domain := matches[2]
	//Remove trailing dot
	domainPrefix := matches[1][:len(matches[1])-1]

	domainsService := client.Domains
	//FIXME: won't work with more than 1000 records
	paginationOptions := &godo.ListOptions{PerPage: 1000}
	records, _, err := domainsService.Records(context.TODO(), domain, paginationOptions)

	if err != nil {
		//FIXME: case on errors
		return NOT_FOUND, err
	}

	var domainRecord *godo.DomainRecord = nil
	for _, v := range records {
		if v.Type == "A" && v.Name == domainPrefix {
			domainRecord = &v
		}
	}

	if domainRecord == nil {
		return NOT_FOUND, errors.New("Subdomain not found")
	}

	if domainRecord.Data == ip.String() {
		return NOT_MODIFIED, nil
	}

	drer := &godo.DomainRecordEditRequest{Data: ip.String()}
	_, _, err = domainsService.EditRecord(context.TODO(), domain, domainRecord.ID, drer)
	if err == nil {
		return OK, nil
	} else {
		//FIXME case on error
		return INTERNAL_SERVER_ERROR, err
	}
}
