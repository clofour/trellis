# Sidecar

Both containers belong to one task group, so every allocation colocates them. Apply with `trellis jobs apply --file examples/sidecar/trellis.yaml`. The exporter configuration is illustrative: enable nginx's `stub_status` in a custom production image/config.
