# Deployment strategies

- Apply `rolling.yaml`, then change its image: Trellis replaces one allocation at a time and waits for healthy replacement capacity.
- Apply `blue.yaml` and `green.yaml` together. Validate green, then change the external proxy from label `route:shop-blue` to `route:shop-green`; later destroy blue.
- Apply `stable.yaml` and `canary.yaml`. `trellis-proxy-sync -label route:shop-weighted ...` exposes both and passes `trellis/weight` to the template. Increase canary weight after observing it, or destroy it to roll back.

Blue/green and canary are compositions using independent jobs and external routing. Only `recreate` and `rolling` are valid manifest strategy values.
