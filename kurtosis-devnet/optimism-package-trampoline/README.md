This package contains a mostly empty Kurtosis package,
that trampolines to github.com/ethpandas/optimism-package.

The goal here is to pin dependencies using 2 distinct mechanisms:

- using the `replace` section of `kurtosis.yml` as a lockfile for our kurtosis package dependencies.
- using the content of `docker_lock.star` as a lockfile for our container images dependencies.

This way, we achieve reproducibility for our devnet deployments.