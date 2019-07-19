package cf

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"autoscaler/models"

	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/lager"
	gcf "github.com/cloudfoundry-community/go-cfclient"
	"github.com/pkg/errors"
)

const (
	runningState = "RUNNING"
)
const (
	PathCFInfo                                   = "/v2/info"
	PathCFAuth                                   = "/oauth/token"
	GrantTypeClientCredentials                   = "client_credentials"
	GrantTypeRefreshToken                        = "refresh_token"
	TimeToRefreshBeforeTokenExpire time.Duration = 10 * time.Minute
)

type Tokens struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type Config struct {
	API               string `yaml:"api"`
	ClientID          string `yaml:"client_id"`
	Secret            string `yaml:"secret"`
	RoutingAPI        string `yaml:"routing_api"`
	SkipSSLValidation bool   `yaml:"skip_ssl_validation"`
}

type Client interface {
	GetToken() (string, error)
	StartApp(appID string) error
	StopApp(appID string) error
	GetApp(appID string) (gcf.App, error)
	SetAppInstance(appID string, instances int) error
	GetAppRoutes(appID string) ([]string, error)
	RegisterRoute(route gcf.Route, ip string, port, ttl int) error
	UnRegisterRoute(route gcf.Route, ip string, port int) error
	RegisterRoutes(routes []models.RouteEntity) error
	UnRegisterRoutes(routes []models.RouteEntity) error
	GetSharedDomainByGuid(domainID string) (gcf.SharedDomain, error)
	GetAppRunningInstanceNumber(appID string) (int, error)
	GetAppSummary(appID string) (gcf.AppSummary, error)
	IsUserAdmin(userToken string) (bool, error)
	IsUserSpaceDeveloper(userToken string, appId string) (bool, error)
}

type client struct {
	config      *Config
	goCFClient  *gcf.Client
	httpClient  *http.Client
	lock        *sync.RWMutex
	domainCache map[string]string
	logger      lager.Logger
}

func NewClient(conf *Config, logger lager.Logger) (Client, error) {
	httpClient := cfhttp.NewClient()
	httpClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: conf.SkipSSLValidation}
	httpClient.Transport.(*http.Transport).DialContext = (&net.Dialer{
		Timeout: 30 * time.Second,
	}).DialContext

	c := &client{
		config:      conf,
		lock:        &sync.RWMutex{},
		domainCache: map[string]string{},
		logger:      logger.Session("cf"),
		httpClient:  httpClient,
	}

	gcfConfig := &gcf.Config{
		ApiAddress:        conf.API,
		ClientID:          conf.ClientID,
		ClientSecret:      conf.Secret,
		SkipSslValidation: conf.SkipSSLValidation,
		HttpClient:        httpClient,
	}
	gcfClient, err := gcf.NewClient(gcfConfig)
	if err != nil {
		return nil, err
	}
	c.goCFClient = gcfClient
	return c, nil
}

func (c *client) GetToken() (string, error) {
	return c.goCFClient.GetToken()
}

func (c *client) GetApp(appID string) (gcf.App, error) {
	return c.goCFClient.GetAppByGuid(appID)
}

func (c *client) StartApp(appID string) error {
	return c.goCFClient.StartApp(appID)
}

func (c *client) StopApp(appID string) error {
	return c.goCFClient.StopApp(appID)
}

func (c *client) SetAppInstance(appID string, instances int) error {
	aur := gcf.AppUpdateResource{
		Instances: instances,
	}
	_, err := c.goCFClient.UpdateApp(appID, aur)
	return err
}
func (c *client) GetAppRunningInstanceNumber(appID string) (int, error) {
	instances, err := c.goCFClient.GetAppInstances(appID)
	if err != nil {
		return -1, err
	}
	count := 0
	for _, instance := range instances {
		if instance.State == runningState {
			count++
		}
	}
	return count, nil
}
func (c *client) GetAppSummary(appID string) (gcf.AppSummary, error) {
	req, err := http.NewRequest(http.MethodGet, c.config.API+"/v2/apps/"+appID+"/summary", nil)
	if err != nil {
		return gcf.AppSummary{}, err
	}
	token, err := c.goCFClient.GetToken()
	if err != nil {
		return gcf.AppSummary{}, err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return gcf.AppSummary{}, err
	}
	resBody, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return gcf.AppSummary{}, errors.Wrap(err, "Error reading app summary body")
	}
	var appSummary gcf.AppSummary
	err = json.Unmarshal(resBody, &appSummary)
	if err != nil {
		return gcf.AppSummary{}, errors.Wrap(err, "Error unmarshalling app summary")
	}
	return appSummary, nil
}
