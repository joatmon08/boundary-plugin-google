data "google_compute_zones" "available" {}

resource "google_compute_instance" "default" {
  count        = var.num_instances
  name         = "boundary-${count.index}"
  machine_type = "n2-standard-2"
  zone         = data.google_compute_zones.available.names[0]

  labels = var.labels

  boot_disk {
    initialize_params {
      image  = "debian-cloud/debian-11"
      labels = var.labels
    }
  }

  network_interface {
    network = "default"

    access_config {
      // Ephemeral public IP
    }
  }

  dynamic "service_account" {
    for_each = var.service_account_email != null ? [var.service_account_email] : []
    content {
      email  = service_account.value
      scopes = ["cloud-platform"]
    }
  }
}

resource "google_compute_instance_group" "webservers" {
  name        = "boundary-servers"
  description = "Boundary server group"

  instances = google_compute_instance.default.*.id

  zone = data.google_compute_zones.available.names[0]
}