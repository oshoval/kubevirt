#!/usr/bin/env python3
#
# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2026 Red Hat, Inc.
#

"""Roll out only KubeVirt workloads whose Bazel image has changed."""

from __future__ import annotations

import json
import os
import shlex
import subprocess
import sys
from urllib.error import HTTPError, URLError
from urllib.parse import urlsplit
from urllib.request import Request, urlopen

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
KUBECTL = "./kubevirtci/cluster-up/kubectl.sh"
TIMEOUT = "10m"

# virt-launcher is staged by virt-handler's init container. virt-exportserver
# has no standing pod; the controller uses its tag when creating export pods.
WORKLOADS = {
    "virt-operator": "deployment/virt-operator",
    "virt-api": "deployment/virt-api",
    "virt-controller": "deployment/virt-controller",
    "virt-handler": "daemonset/virt-handler",
    "virt-launcher": "daemonset/virt-handler",
    "virt-exportserver": None,
    "virt-exportproxy": "deployment/virt-exportproxy",
    "virt-synchronization-controller": "deployment/virt-synchronization-controller",
}
FEATURE_GATES = {"virt-synchronization-controller": "DecentralizedLiveMigration"}


def run(
    args: list[str], *, env: dict[str, str] | None = None, capture: bool = True
) -> str:
    result = subprocess.run(
        args,
        cwd=ROOT,
        env=env,
        text=True,
        stdout=subprocess.PIPE if capture else None,
    )
    if result.returncode:
        raise RuntimeError(f"Command failed ({result.returncode}): {shlex.join(args)}")
    return result.stdout or ""


def kubectl(env, *args) -> str:
    return run([KUBECTL, *args], env=env)


def kubectl_json(env, *args) -> dict:
    return json.loads(kubectl(env, *args))


def project_env(env: dict[str, str]) -> dict[str, str]:
    script = '''
set -e
source hack/common.sh
source "kubevirtci/cluster-up/cluster/${KUBEVIRT_PROVIDER}/provider.sh"
source hack/config.sh
printf '%s\\0' KUBEVIRT_PROVIDER "${KUBEVIRT_PROVIDER-}" docker_prefix "${docker_prefix-}" image_prefix "${image_prefix-}" ARCHITECTURE "${ARCHITECTURE-}"
'''
    fields = run(["bash", "-c", script], env=env).split("\0")
    values = dict(zip(fields[::2], fields[1::2]))
    return {**env, **values}


def select_images(env: dict[str, str]) -> list[str]:
    raw_requested = env.get("PUSH_TARGETS", "")
    requested = raw_requested.replace(",", " ").split()
    use_hints = env.get("USE_HINTS", "true")
    if use_hints not in {"true", "false"}:
        raise RuntimeError("USE_HINTS must be true or false")
    if raw_requested and not requested:
        raise RuntimeError("PUSH_TARGETS contains no image names")
    unknown = set(requested) - WORKLOADS.keys()
    if unknown:
        raise RuntimeError(f"Unknown resync images: {' '.join(sorted(unknown))}")
    if not requested:
        return list(WORKLOADS)
    if "virt-handler" in requested and use_hints == "true" and "virt-launcher" not in requested:
        requested.append("virt-launcher")
    return requested


def validate_cluster_architecture(env: dict[str, str]) -> None:
    cluster_architectures = {
        node.get("status", {}).get("nodeInfo", {}).get("architecture")
        for node in kubectl_json(env, "get", "nodes", "-o", "json").get("items", [])
    }
    if not cluster_architectures or None in cluster_architectures:
        raise RuntimeError("Cannot determine the architecture of every cluster node")
    if len(cluster_architectures) != 1:
        raise RuntimeError("Resync requires all cluster nodes to use the same architecture")

    build_arch = env.get("ARCHITECTURE", "").removeprefix("crossbuild-").removeprefix("cross-")
    build_arch = {"x86_64": "amd64", "aarch64": "arm64"}.get(build_arch, build_arch)
    cluster_arch = next(iter(cluster_architectures))
    if build_arch != cluster_arch:
        raise RuntimeError(
            f"Build architecture {build_arch} does not match cluster architecture {cluster_arch}"
        )


def registry_digest(env: dict[str, str], image: str, tag: str) -> str:
    prefix = env.get("DOCKER_PREFIX") or env.get("docker_prefix", "")
    repository_prefix = env.get("IMAGE_PREFIX") or env.get("image_prefix", "")
    parsed = urlsplit("http://" + prefix.removeprefix("http://").removeprefix("https://"))
    local_address = env.get("BAZEL_REGISTRY_LOCAL_ADDRESS") or (
        f"127.0.0.1:{env.get('BAZEL_REGISTRY_PORT', '5001')}"
    )
    local_netloc = urlsplit(f"http://{local_address}").netloc
    repository = "/".join(
        part.strip("/")
        for part in (parsed.path, repository_prefix, image)
        if part.strip("/")
    )
    url = f"http://{local_netloc}/v2/{repository}/manifests/{tag}"
    request = Request(
        url,
        method="HEAD",
        headers={
            "Accept": ", ".join(
                (
                    "application/vnd.oci.image.manifest.v1+json",
                    "application/vnd.docker.distribution.manifest.v2+json",
                )
            )
        },
    )
    try:
        with urlopen(request, timeout=10) as response:
            digest = response.headers.get("Docker-Content-Digest", "")
    except (HTTPError, URLError) as error:
        raise RuntimeError(f"Cannot read Bazel registry manifest for {image}:{tag}: {error}") from error
    if not digest.startswith("sha256:"):
        raise RuntimeError(f"Registry returned no manifest digest for {image}:{tag}")
    return digest


def digest_from_image_id(image_id: str) -> str:
    return image_id.rsplit("@", 1)[-1].removeprefix("containerd://").removeprefix("docker://")


def workload_matches(
    workload: dict, pods: list[dict], image: str, tag: str, digest: str
) -> bool:
    selector = workload.get("spec", {}).get("selector", {}).get("matchLabels", {})
    if not selector:
        return False
    selected = [
        pod
        for pod in pods
        if not pod.get("metadata", {}).get("deletionTimestamp")
        if all(
            pod.get("metadata", {}).get("labels", {}).get(key) == value
            for key, value in selector.items()
        )
    ]
    status = workload.get("status", {})
    kind = workload.get("kind")
    if kind == "DaemonSet":
        desired = status.get("desiredNumberScheduled", 0)
        ready = status.get("numberReady", 0)
        updated = status.get("updatedNumberScheduled", 0)
    else:
        desired = workload.get("spec", {}).get("replicas") or 1
        ready = status.get("readyReplicas", 0)
        updated = status.get("updatedReplicas", 0)
    observed = status.get("observedGeneration", 0)
    generation = workload.get("metadata", {}).get("generation", 0)
    if (
        not desired
        or ready != desired
        or updated != desired
        or observed < generation
        or len(selected) != desired
    ):
        return False
    for pod in selected:
        state = pod.get("status", {})
        ready = any(
            condition.get("type") == "Ready" and condition.get("status") == "True"
            for condition in state.get("conditions", [])
        )
        if state.get("phase") != "Running" or not ready:
            return False
        spec = pod.get("spec", {})
        containers = spec.get("containers", []) + spec.get("initContainers", [])
        statuses = {
            status.get("name"): status
            for status in state.get("containerStatuses", [])
            + state.get("initContainerStatuses", [])
        }
        matches = [
            container
            for container in containers
            if container.get("image", "").rsplit("/", 1)[-1].split(":", 1)[0]
            == image
        ]
        if not matches or any(
            container.get("image", "").rsplit("/", 1)[-1] != f"{image}:{tag}"
            or digest_from_image_id(
                statuses.get(container.get("name"), {}).get("imageID", "")
            )
            != digest
            for container in matches
        ):
            return False
    return True


def replace_workload(env, namespace: str, workload_ref: str) -> None:
    workload = kubectl_json(env, "-n", namespace, "get", workload_ref, "-o", "json")
    selector = workload.get("spec", {}).get("selector", {})
    if selector.get("matchExpressions"):
        raise RuntimeError(f"Cannot safely restart {workload_ref}: selector uses expressions")
    labels = selector.get("matchLabels", {})
    label_selector = ",".join(f"{key}={value}" for key, value in labels.items())
    if not label_selector:
        raise RuntimeError(f"Cannot safely restart {workload_ref}: selector is empty")

    status = workload.get("status", {})
    if workload.get("kind") == "DaemonSet":
        expected = status.get("desiredNumberScheduled", 0)
        ready_path = ".status.numberReady"
    else:
        expected = workload.get("spec", {}).get("replicas") or 1
        ready_path = ".status.readyReplicas"
    if not expected:
        raise RuntimeError(f"Cannot safely restart {workload_ref}: it has no desired pods")

    old_pods = kubectl_json(
        env, "-n", namespace, "get", "pods", "-l", label_selector, "-o", "json"
    ).get("items", [])
    pod_refs = [
        f"pod/{pod['metadata']['name']}"
        for pod in old_pods
        if pod.get("metadata", {}).get("name")
    ]
    pods_to_delete = [
        f"pod/{pod['metadata']['name']}"
        for pod in old_pods
        if pod.get("metadata", {}).get("name")
        and not pod.get("metadata", {}).get("deletionTimestamp")
    ]
    if pods_to_delete:
        # Resync is a development workflow; replace the workload together and
        # wait once instead of preserving availability pod by pod.
        kubectl(env, "-n", namespace, "delete", *pods_to_delete, "--wait=false")
    if pod_refs:
        kubectl(
            env,
            "-n",
            namespace,
            "wait",
            *pod_refs,
            "--for=delete",
            f"--timeout={TIMEOUT}",
        )
    kubectl(
        env,
        "-n",
        namespace,
        "wait",
        f"--for=jsonpath={{{ready_path}}}={expected}",
        workload_ref,
        f"--timeout={TIMEOUT}",
    )


def resync(env: dict[str, str]) -> None:
    env = project_env(env)
    namespace = env.get("KUBEVIRT_INSTALLED_NAMESPACE", "kubevirt")
    prefix = env.get("DOCKER_PREFIX", "").rstrip("/")
    advertise = env.get("BAZEL_REGISTRY_ADVERTISE", "")
    if not prefix or not advertise:
        raise RuntimeError("Set DOCKER_PREFIX and BAZEL_REGISTRY_ADVERTISE for the reachable Bazel registry")
    operator = kubectl(
        env,
        "-n",
        namespace,
        "get",
        "deployment/virt-operator",
        '-o=jsonpath={.spec.template.spec.containers[?(@.name=="virt-operator")].image}',
    ).strip()
    if not operator:
        raise RuntimeError(f"No virt-operator found in namespace {namespace}")
    tag = env.get("DOCKER_TAG") or operator.rsplit(":", 1)[-1]
    installed_tag = operator.rsplit(":", 1)[-1]
    if tag != installed_tag:
        raise RuntimeError(
            f"DOCKER_TAG={tag} differs from the installed operator tag {installed_tag}; "
            "run no-push cluster-sync first"
        )
    images = select_images(env)
    if env.get("KUBEVIRT_CROSS_ARCH_EMULATION") == "true":
        raise RuntimeError("Resync does not support cross-architecture emulation images")
    validate_cluster_architecture(env)
    current = kubectl_json(
        env, "-n", namespace, "get", "deployments,daemonsets", "-o", "json"
    )["items"]
    current_workloads = {
        f"{resource['kind'].lower()}/{resource['metadata']['name']}": resource
        for resource in current
    }
    kubevirt = kubectl_json(env, "-n", namespace, "get", "kv/kubevirt", "-o", "json")
    enabled_gates = set(
        kubevirt.get("spec", {})
        .get("configuration", {})
        .get("developerConfiguration", {})
        .get("featureGates", [])
    )
    image_prefix = env.get("IMAGE_PREFIX") or env.get("image_prefix", "")
    for image in images:
        workload_ref = WORKLOADS[image]
        gate = FEATURE_GATES.get(image)
        if not workload_ref or (gate and gate not in enabled_gates):
            continue
        workload = current_workloads.get(workload_ref)
        if not workload:
            continue
        expected = f"{prefix}/{image_prefix}{image}:{tag}"
        pod_spec = workload.get("spec", {}).get("template", {}).get("spec", {})
        deployed_images = [
            container.get("image", "")
            for container in pod_spec.get("containers", [])
            + pod_spec.get("initContainers", [])
        ]
        if expected not in deployed_images:
            raise RuntimeError(
                f"{workload_ref} does not use the Bazel registry image {expected}; "
                "run no-push cluster-sync first"
            )
    # Refresh all served images so unrelated workloads can still pull during rollout.
    assignments = {
        # Keep the endpoint complete, since unrelated workloads may pull while
        # a selected workload is rolling out.
        "BAZEL_IMAGE_REGISTRY": "1",
        "BAZEL_IMAGE_REGISTRY_BUILD_SELECTED_ONLY": "1",
        "BUILD_ARCH": env.get("BUILD_ARCH", ""),
        "DOCKER_PREFIX": prefix,
        "DOCKER_TAG": tag,
        "KUBEVIRT_PROVIDER": env["KUBEVIRT_PROVIDER"],
        "PUSH_TARGETS": " ".join(images),
        "IMAGE_PREFIX": env.get("IMAGE_PREFIX", env.get("image_prefix", "")),
        "BAZEL_REGISTRY_ADVERTISE": advertise,
        "BAZEL_REGISTRY_LISTEN_ADDRESS": env.get("BAZEL_REGISTRY_LISTEN_ADDRESS", "0.0.0.0"),
        "BAZEL_REGISTRY_PORT": env.get("BAZEL_REGISTRY_PORT", "5001"),
    }
    command_prefix = " ".join(
        f"{key}={shlex.quote(value)}" for key, value in assignments.items()
    )
    run(
        ["./hack/dockerized", f"{command_prefix} ./hack/bazel-push-images.sh"],
        env=env,
        capture=False,
    )

    resources = kubectl_json(env, "-n", namespace, "get", "deployments,daemonsets,pods", "-o", "json")["items"]
    workloads = {
        f"{resource['kind'].lower()}/{resource['metadata']['name']}": resource
        for resource in resources
        if resource.get("kind") in {"Deployment", "DaemonSet"}
    }
    pods = [resource for resource in resources if resource.get("kind") == "Pod"]
    changed = {}
    digests = {}
    for image in images:
        workload_ref = WORKLOADS[image]
        digest = registry_digest(env, image, tag)
        digests[image] = digest
        gate = FEATURE_GATES.get(image)
        if gate and gate not in enabled_gates:
            print(f"Skipping {image}: feature gate {gate} is disabled")
            continue
        if workload_ref is None:
            print(f"Checked {image} in Bazel registry; no standing workload to restart")
            continue
        workload = workloads.get(workload_ref)
        if workload is None:
            print(f"Skipping {image}: {workload_ref} is not installed")
            continue
        if not workload_matches(workload, pods, image, tag, digest):
            changed.setdefault(workload_ref, []).append(image)

    # The operator reconciles the KubeVirt workloads; restart it first.
    changed = sorted(changed.items(), key=lambda item: item[0] != "deployment/virt-operator")
    if not changed:
        print("No-op: selected images match the running workloads.")
    for workload_ref, images_for_workload in changed:
        print(f"Rolling out {', '.join(images_for_workload)} via {workload_ref}")
        replace_workload(env, namespace, workload_ref)

    kubectl(
        env,
        "-n",
        namespace,
        "wait",
        "kv/kubevirt",
        "--for=condition=Available=True",
        f"--timeout={TIMEOUT}",
    )
    # Re-read pod status after reconciliation and fail if any selected image is stale.
    final_resources = kubectl_json(
        env, "-n", namespace, "get", "deployments,daemonsets,pods", "-o", "json"
    )["items"]
    final_workloads = {
        f"{resource['kind'].lower()}/{resource['metadata']['name']}": resource
        for resource in final_resources
        if resource.get("kind") in {"Deployment", "DaemonSet"}
    }
    final_pods = [resource for resource in final_resources if resource.get("kind") == "Pod"]
    for image in images:
        workload_ref = WORKLOADS[image]
        if image not in digests or workload_ref is None:
            continue
        gate = FEATURE_GATES.get(image)
        if gate and gate not in enabled_gates:
            continue
        workload = final_workloads.get(workload_ref)
        if workload and not workload_matches(workload, final_pods, image, tag, digests[image]):
            raise RuntimeError(f"Final validation failed for {image}")
    print(f"Validated {len(digests)} image digest(s); rolled out {len(changed)} workload(s).")


if __name__ == "__main__":
    try:
        resync(os.environ.copy())
    except (RuntimeError, KeyError, json.JSONDecodeError) as error:
        print(error, file=sys.stderr)
        sys.exit(1)
