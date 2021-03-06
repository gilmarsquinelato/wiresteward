# wiresteward terraform modules

This is a collection of terraform modules describing the recommended deployment
for wiresteward.

They are based on [Flatcar Container Linux](https://www.flatcar-linux.org/).

## Example usage

For example, to deploy on aws:

```
locals {
  hostname_base = "wiresteward.example.com"
}

module "wiresteward_ignition" {
  source = "github.com/utilitywarehouse/wiresteward//terraform/ignition?ref=master"

  oauth2_client_id           = "xxxxxxxxxxxxxxxxxxxxx"
  oauth2_introspect_url      = "https://login.uw.systems/oauth2/default/v1/introspect"
  wireguard_cidrs            = ["10.10.0.1/24", "10.10.0.2/24"]
  wireguard_endpoint_base    = local.hostname_base
  wireguard_exposed_subnets  = ["10.20.0.0/16"]
}

module "wiresteward" {
  source = "github.com/utilitywarehouse/wiresteward//terraform/aws?ref=master"

  dns_zone_id          = "/hostedzone/XXXXXXXXXXXXX"
  ignition             = module.wiresteward_ignition.ignition
  wireguard_endpoints  = module.wiresteward_ignition.endpoints
  wiresteward_endpoint = local.hostname_base
  subnet_ids           = ["subnet-xxxxxxxx", "subnet-xxxxxxxx", "subnet-xxxxxxxx"]
  vpc_id               = "vpc-xxxxxxxxxxxxxxxxx"
}
```

- `wireguard_cidr` defines the IP address of the server peer and the subnets from
which IP addresses are allocated to user peers. The length of the list
determines the number of instances launched.

- `wireguard_exposed_subnets` lists the subnets to which the server peer will
forward traffic.
