locals {
  startup_script = templatefile("${path.module}/startup.sh.tftpl", {
    repo_url         = var.repo_url
    repo_ref         = var.repo_ref
    app_port         = var.app_port
    postgres_password = var.postgres_password
    app_root         = "/opt/workflow-engine"
    repo_dir         = "workspace"
  })
}

resource "google_compute_network" "main" {
  name                    = "${var.name}-network"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "main" {
  name          = "${var.name}-subnet"
  ip_cidr_range = "10.10.0.0/24"
  region        = var.region
  network       = google_compute_network.main.id
}

resource "google_compute_firewall" "app" {
  name    = "${var.name}-allow-app"
  network = google_compute_network.main.name

  allow {
    protocol = "tcp"
    ports    = [tostring(var.app_port)]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags    = ["${var.name}-app"]
}

resource "google_compute_firewall" "ssh" {
  name    = "${var.name}-allow-ssh"
  network = google_compute_network.main.name

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = var.ssh_source_ranges
  target_tags    = ["${var.name}-app"]
}

resource "google_compute_address" "main" {
  name   = "${var.name}-ip"
  region = var.region
}

resource "google_compute_instance" "main" {
  name         = var.name
  machine_type = var.machine_type
  zone         = var.zone
  tags         = ["${var.name}-app"]

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2204-lts"
      size  = 30
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.main.id
    access_config {
      nat_ip = google_compute_address.main.address
    }
  }

  metadata_startup_script = local.startup_script
}
