#!/usr/bin/env bash
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
# Copyright 2017 Red Hat, Inc.
#

# This script is called by cluster-sync.sh

set -e

DOCKER_TAG=${DOCKER_TAG:-devel}
DOCKER_TAG_ALT=${DOCKER_TAG_ALT:-devel_alt}
SKIP_BUILD=${SKIP_BUILD:-}

if [[ -n "$SKIP_BUILD" ]]; then
    PUSH_TARGETS=${PUSH_TARGETS:-"virt-operator virt-api virt-controller virt-handler virt-launcher virt-exportserver virt-exportproxy"}
    DOCKER_TAG_ALT=
fi

source hack/common.sh
source kubevirtci/cluster-up/cluster/$KUBEVIRT_PROVIDER/provider.sh
source hack/config.sh

echo "Building ..."

# Build everything and publish it
if [[ -z "$SKIP_BUILD" ]]; then
    ${KUBEVIRT_PATH}hack/dockerized "BUILD_ARCH=${BUILD_ARCH} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} KUBEVIRT_PROVIDER=${KUBEVIRT_PROVIDER} ./hack/bazel-build-functests.sh"
else
    echo "Skipping build (SKIP_BUILD=$SKIP_BUILD)"
fi
${KUBEVIRT_PATH}hack/dockerized "BUILD_ARCH=${BUILD_ARCH} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} DOCKER_TAG_ALT=${DOCKER_TAG_ALT} KUBEVIRT_PROVIDER=${KUBEVIRT_PROVIDER} IMAGE_PREFIX=${IMAGE_PREFIX} IMAGE_PREFIX_ALT=${IMAGE_PREFIX_ALT} PUSH_TARGETS='${PUSH_TARGETS}' ./hack/multi-arch.sh push-images"
BUILD_ARCH=${BUILD_ARCH} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} hack/push-container-manifest.sh

echo "Done $0"
