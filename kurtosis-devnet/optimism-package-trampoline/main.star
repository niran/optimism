"""
This is a trampoline script that delegates to the optimism-package.
It is used to pin the dependencies to specific versions.
"""

_optimism_package = import_module("github.com/ethpandaops/optimism-package/main.star")
_docker_lock = import_module("docker_lock.star")

def run(plan, args):
    explicit_registry = args.get("registry", {})
    args["registry"] = _docker_lock.PINNED_IMAGES | explicit_registry
    # delegate to optimism-package
    _optimism_package.run(plan, args)
