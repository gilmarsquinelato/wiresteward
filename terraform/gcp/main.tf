data "google_compute_zones" "available" {}

resource "google_compute_instance" "wiresteward" {
  count                     = local.instance_count
  name                      = "${local.name}-${count.index}"
  machine_type              = "e2-micro"
  can_ip_forward            = true
  zone                      = data.google_compute_zones.available.names[count.index]
  allow_stopping_for_update = true

  tags = [local.name]

  boot_disk {
    initialize_params {
      image = "flatcar-beta"
    }
  }

  network_interface {
    subnetwork = var.subnet_link

    access_config {
      // Ephemeral IP
    }
  }

  metadata = {
    user-data = var.ignition[count.index]
  }
}

resource "google_compute_instance_group" "wiresteward" {
  count = local.instance_count
  name  = "${local.name}-${count.index}"

  instances = [
    google_compute_instance.wiresteward.*.id[count.index],
  ]

  named_port {
    name = "oauth2-http"
    port = "4180"
  }

  zone = data.google_compute_zones.available.names[count.index]
}

resource "google_dns_record_set" "wiresteward" {
  count = local.instance_count
  name  = var.wireguard_endpoints[count.index]
  type  = "A"
  ttl   = 30 # TODO increase the value once happy with setup

  managed_zone = var.dns_zone

  # ephemeral ips could change via manual operations on the instance and leave this not updated
  rrdatas = [google_compute_instance.wiresteward[count.index].network_interface.0.access_config.0.nat_ip]
}

resource "google_compute_firewall" "wiresteward-udp" {
  name      = "${local.name}-udp"
  network   = var.vpc_link
  direction = "INGRESS"
  allow {
    protocol = "udp"
    ports    = ["51820"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = [local.name]
}

//https://cloud.google.com/load-balancing/docs/health-checks#firewall_rules
resource "google_compute_firewall" "wiresteward-healthcheck" {
  name    = "${local.name}-healthcheck"
  network = var.vpc_link

  direction = "INGRESS"

  allow {
    protocol = "tcp"
    ports    = ["4180"]
  }

  // https://cloud.google.com/load-balancing/docs/health-checks#fw-netlb
  source_ranges = ["35.191.0.0/16", "130.211.0.0/22"]
  target_tags   = [local.name]
}

resource "google_compute_firewall" "wiresteward-ssh" {
  name    = "${local.name}-ssh"
  network = var.vpc_link

  direction = "INGRESS"

  allow {
    protocol = "tcp"
    ports    = ["50620"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = [local.name]
}
