#!/usr/bin/env bash
set -euo pipefail

apt-get update
apt-get install -y ca-certificates curl avahi-daemon libnss-mdns
systemctl enable --now avahi-daemon
