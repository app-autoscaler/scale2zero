package cf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"autoscaler/models"

	"code.cloudfoundry.org/lager"
	gcf "github.com/cloudfoundry-community/go-cfclient"
)

const PATH_HTTP_ROUTES = "/v1/routes"

type RouteResource struct {
	Route string `json:"route"`
	IP    string `json:ip`
	Port  int    `json:port`
	TTL   int    `json:ttl,omitempty`
}

func (c *client) GetSharedDomainByGuid(domainID string) (gcf.SharedDomain, error) {
	return c.goCFClient.GetSharedDomainByGuid(domainID)
}

func (c *client) GetAppRoutes(appID string) ([]string, error) {
	routes, err := c.goCFClient.GetAppRoutes(appID)
	if err != nil {
		return nil, err
	}

	app, err := c.goCFClient.GetAppByGuidNoInlineCall(appID)
	if err != nil {
		return nil, err
	}

	appRoutes := []string{}
	for _, route := range routes {
		fmt.Printf("%+v", route)
		if route.Port != 0 {
			c.logger.Info("tcp-route-not-support", lager.Data{"appID": appID, "route": route})
			continue
		}
		c.lock.RLock()
		domainName, exists := c.domainCache[route.DomainGuid]
		c.lock.RUnlock()
		if !exists {
			sharedDomain, err := c.goCFClient.GetSharedDomainByGuid(route.DomainGuid)
			if err == nil {
				domainName = sharedDomain.Name
			} else {
				privateDomains, err := app.SpaceData.Entity.OrgData.Entity.ListPrivateDomains()
				if err != nil {
					return nil, err
				}
				for _, domain := range privateDomains {
					if domain.Guid == route.DomainGuid {
						domainName = domain.Name
						break
					}
				}
			}
			if domainName == "" {
				return nil, fmt.Errorf("%s: %s", "Can not or failed to find domain with ID", route.DomainGuid)
			}
			c.lock.Lock()
			c.domainCache[route.DomainGuid] = domainName
			c.lock.Unlock()
		}
		routeStr := domainName
		if route.Host != "" {
			routeStr = fmt.Sprintf("%s.%s", route.Host, routeStr)
		}
		if route.Path != "" {
			routeStr = routeStr + route.Path
		}
		appRoutes = append(appRoutes, routeStr)
	}

	return appRoutes, nil
}

func (c *client) RegisterRoute(route gcf.Route, ip string, port, ttl int) error {
	routeRsc := RouteResource{
		Route: route.Host + route.Path,
		IP:    ip,
		Port:  port,
		TTL:   ttl,
	}
	body, err := json.Marshal(routeRsc)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.config.RoutingAPI+PATH_HTTP_ROUTES, bytes.NewReader(body))
	if err != nil {
		return err
	}

	token, err := c.goCFClient.GetToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Route create API returned with status code %d", resp.StatusCode)
	}
	return nil
}
func (c *client) RegisterRoutes(routes []models.RouteEntity) error {
	body, err := json.Marshal(routes)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.config.RoutingAPI+PATH_HTTP_ROUTES, bytes.NewReader(body))
	if err != nil {
		return err
	}

	token, err := c.goCFClient.GetToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Route create API returned with status code %d", resp.StatusCode)
	}
	return nil
}
func (c *client) UnRegisterRoute(route gcf.Route, ip string, port int) error {
	routeRsc := RouteResource{
		Route: route.Host + route.Path,
		IP:    ip,
		Port:  port,
	}
	body, err := json.Marshal(routeRsc)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodDelete, c.config.RoutingAPI+PATH_HTTP_ROUTES, bytes.NewReader(body))
	if err != nil {
		return err
	}

	token, err := c.goCFClient.GetToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Route delete API returned with status code %d", resp.StatusCode)
	}
	return nil
}
func (c *client) UnRegisterRoutes(routes []models.RouteEntity) error {
	// routeRsc := RouteResource{
	// 	Route: route.Host + route.Path,
	// 	IP:    ip,
	// 	Port:  port,
	// }
	body, err := json.Marshal(routes)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodDelete, c.config.RoutingAPI+PATH_HTTP_ROUTES, bytes.NewReader(body))
	if err != nil {
		return err
	}

	token, err := c.goCFClient.GetToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Route delete API returned with status code %d", resp.StatusCode)
	}
	return nil
}
