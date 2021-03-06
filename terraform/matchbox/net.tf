data "ignition_networkd_unit" "bond_net_eno" {
  name    = "00-eno.network"
  content = <<EOS
[Match]
Name=eno*

[Link]
MTUBytes=9000

[Network]
Bond=bond0
EOS
}

data "ignition_networkd_unit" "bond_netdev" {
  name    = "10-bond0.netdev"
  content = <<EOS
[NetDev]
Name=bond0
Kind=bond

[Bond]
Mode=802.3ad
EOS
}

data "ignition_networkd_unit" "bond_private_vlan_netdev" {
  count = length(var.wiresteward_server_peers)

  name    = "11-bond-private-vlan.netdev"
  content = <<EOS
[NetDev]
Name=bond0.${var.private_vlan_id}
Kind=vlan

[VLAN]
Id=${var.private_vlan_id}
EOS
}

data "ignition_networkd_unit" "bond_public_vlan_netdev" {
  count = length(var.wiresteward_server_peers)

  name    = "12-bond-public-vlan.netdev"
  content = <<EOS
[NetDev]
Name=bond0.${var.public_vlan_id}
Kind=vlan

[VLAN]
Id=${var.public_vlan_id}
EOS
}

data "ignition_networkd_unit" "bond0" {
  count = length(var.wiresteward_server_peers)

  name    = "20-bond0.network"
  content = <<EOS
[Match]
Name=bond0
[Link]
MTUBytes=9000
MACAddress=${var.wiresteward_server_peers[count.index].mac_addresses[0]}
[Network]
DHCP=no
DNS=1.1.1.1
VLAN=bond0.${var.private_vlan_id}
VLAN=bond0.${var.public_vlan_id}
EOS
}

data "ignition_networkd_unit" "bond0_private_vlan" {
  count = length(var.wiresteward_server_peers)

  name = "21-bond0-private-vlan.network"

  content = <<EOS
[Match]
Name=bond0.${var.private_vlan_id}
[Network]
DHCP=no
[Address]
Address=${var.wiresteward_server_peers[count.index].private_ip_address}/24
[Route]
Destination=10.0.0.0/8
Gateway=${var.private_vlan_gw}
EOS
}

data "ignition_networkd_unit" "bond0_public_vlan" {
  count = length(var.wiresteward_server_peers)

  name = "22-bond0-public-vlan.network"

  content = <<EOS
[Match]
Name=bond0.${var.public_vlan_id}
[Network]
DHCP=no
[Address]
Address=${var.wiresteward_server_peers[count.index].public_ip_address}/28
[Route]
Destination=0.0.0.0/0
Gateway=${var.public_vlan_gw}
EOS
}
