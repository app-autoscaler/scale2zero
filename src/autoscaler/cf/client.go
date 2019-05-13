package cf

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"

	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/lager"
	gcf "github.com/cloudfoundry-community/go-cfclient"
)

type Config struct {
	API               string `yaml:"api"`
	ClientID          string `yaml:"client_id"`
	Secret            string `yaml:"secret"`
	RoutingAPI        string `yaml:routing_api`
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
	GetSharedDomainByGuid(domainID string) (gcf.SharedDomain, error)
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
	c := &client{
		config:      conf,
		lock:        &sync.RWMutex{},
		domainCache: map[string]string{},
		logger:      logger.Session("cf"),
	}
	httpClient := cfhttp.NewClient()
	httpClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: conf.SkipSSLValidation}
	httpClient.Transport.(*http.Transport).DialContext = (&net.Dialer{
		Timeout: 30 * time.Second,
	}).DialContext

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
