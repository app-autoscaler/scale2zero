package main

import (
	"autoscaler/cf"
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"
)

func main() {
	conf := &cf.Config{
		API:               "https://api.bosh-lite.com",
		ClientID:          "autoscaler_client_id",
		Secret:            "autoscaler_client_secret",
		RoutingAPI:        "https://api.bosh-lite.com/routing",
		SkipSSLValidation: true,
	}

	logger := lager.NewLogger("autoscaler")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	client, err := cf.NewClient(conf, logger)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	routes, err := client.GetAppRoutes("6b4d36f3-bcc8-4a3b-9a95-09d123836273")
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Printf("%+v\n", routes)

}
