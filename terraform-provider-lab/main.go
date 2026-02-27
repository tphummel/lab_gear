package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/tomflanagan/terraform-provider-lab/internal/provider"
)

func main() {
	err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/tomflanagan/lab",
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
