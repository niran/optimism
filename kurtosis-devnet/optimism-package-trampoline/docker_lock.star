_registry = import_module("github.com/ethpandaops/optimism-package/src/package_io/registry.star")

PINNED_IMAGES = {
    _registry.PROXYD: "us-docker.pkg.dev/oplabs-tools-artifacts/images/proxyd:v4.15.0",
}
