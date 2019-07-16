package main

import (
	"autoscaler/cf"
	"autoscaler/models"
	"code.cloudfoundry.org/lager"
	"fmt"
)

func main() {
	cfConfig := &cf.Config{
		API:               "https://api.bosh-lite.com",
		ClientID:          "autoscaler_client_id",
		Secret:            "autoscaler_client_secret",
		RoutingAPI:        "https://api.bosh-lite.com/routing",
		SkipSSLValidation: true,
	}
	client, err := cf.NewClient(cfConfig, lager.NewLogger("test"))
	if err != nil {
		fmt.Printf("create client=======>%s\n", err)
		return
	}
	summary, err := client.GetAppSummary("c7d54a5e-e746-48e1-80db-eda6653b81ac")
	fmt.Printf("summary:%v\n", summary)
	fmt.Printf("err:%v\n", err)
}
