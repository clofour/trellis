# Patroni

This is an architecture example, not a turnkey PostgreSQL cluster. It demonstrates three independently placed replicas, WireGuard networking, Trellis API discovery, node-local storage, secrets, rolling replacement, and Patroni REST health checks.

Before use:

1. Provision at least three nodes labeled `database=true`, each advertising a separately backed-up `patroni-data` host volume.
2. Create `postgres-password` and `replication-password` in namespace `database`.
3. Build a pinned Patroni image/config that uses a supported distributed configuration store. Trellis discovery (`discover-members.sh`) can locate endpoints, but the Trellis catalog is **not** Patroni's consensus DCS.
4. Give every member a unique Patroni name; the static environment value is only a placeholder that your entrypoint should derive from allocation identity.
5. Test bootstrap, synchronous/asynchronous policy, leader loss, fencing, rewind, backups, point-in-time recovery, and upgrades before carrying data.

Apply only after those changes:

```sh
trellis --namespace database jobs apply --file examples/patroni/trellis.yaml
```

Trellis schedules and monitors containers; Patroni/PostgreSQL still own database replication and promotion correctness.
