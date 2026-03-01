package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/tphummel/lab_gear/terraform-provider-lab_gear/internal/provider"
)

func main() {
	err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		// Address must match the source in consumers' required_providers block.
		Address: "registry.terraform.io/tphummel/lab_gear",
	})
	if err != nil {
		log.Fatal(err)
	}
}
