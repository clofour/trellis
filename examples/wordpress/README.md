# WordPress

Provision one node with `wordpress-db` and `wordpress-content`, create `wordpress-db-password` and `mariadb-root-password`, and apply the manifest. Both tasks are colocated, so WordPress reaches MariaDB on loopback. This compact topology is educational; production deployments need tested database backup/restore and failure-domain planning.
