# Volumes

Provision a node with label `storage=fast` and advertised host volume `app-data`, then apply this manifest. `scratch` is allocation-local; `database` maps an operator-managed node volume. Trellis does not replicate or back up host volume contents.
