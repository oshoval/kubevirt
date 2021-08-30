#!/bin/bash
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

# CI considerations: $TARGET is used by the jenkins build, to distinguish what to test
# Currently considered $TARGET values:
#     vagrant-dev: Runs all functional tests on a development vagrant setup (deprecated)
#     vagrant-release: Runs all possible functional tests on a release deployment in vagrant (deprecated)
#     kubernetes-dev: Runs all functional tests on a development kubernetes setup
#     kubernetes-release: Runs all functional tests on a release kubernetes setup
#     openshift-release: Runs all functional tests on a release openshift setup
#     TODO: vagrant-tagged-release: Runs all possible functional tests on a release deployment in vagrant on a tagged release

set -ex

export TIMESTAMP=${TIMESTAMP:-1}

export WORKSPACE="${WORKSPACE:-$PWD}"
readonly ARTIFACTS_PATH="${ARTIFACTS-$WORKSPACE/exported-artifacts}"
readonly TEMPLATES_SERVER="https://templates.ovirt.org/kubevirt/"
readonly BAZEL_CACHE="${BAZEL_CACHE:-http://bazel-cache.kubevirt-prow.svc.cluster.local:8080/kubevirt.io/kubevirt}"

if [[ $TARGET =~ windows.* ]]; then
  echo "picking the default provider for windows tests"
elif [[ $TARGET =~ cnao ]]; then
  export KUBEVIRT_WITH_CNAO=true
  export KUBEVIRT_PROVIDER=${TARGET/-cnao/}
elif [[ $TARGET =~ sig-network ]]; then
  export KUBEVIRT_WITH_CNAO=true
  export KUBEVIRT_PROVIDER=${TARGET/-sig-network/}
else
  export KUBEVIRT_PROVIDER=${TARGET}
fi

export RHEL_NFS_DIR=${RHEL_NFS_DIR:-/var/lib/stdci/shared/kubevirt-images/rhel7}
export RHEL_LOCK_PATH=${RHEL_LOCK_PATH:-/var/lib/stdci/shared/download_rhel_image.lock}
export WINDOWS_NFS_DIR=${WINDOWS_NFS_DIR:-/var/lib/stdci/shared/kubevirt-images/windows2016}
export WINDOWS_LOCK_PATH=${WINDOWS_LOCK_PATH:-/var/lib/stdci/shared/download_windows_image.lock}


kubectl() { cluster-up/kubectl.sh "$@"; }

export NAMESPACE="${NAMESPACE:-kubevirt}"

# Build and test images with a custom image name prefix
export IMAGE_PREFIX_ALT=${IMAGE_PREFIX_ALT:-kv-}

ginko_params="--noColor --seed=42"

# Prepare PV for Windows testing
if [[ $TARGET =~ windows.* ]]; then
  kubectl create -f - <<EOF
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: disk-windows
  labels:
    kubevirt.io/test: "windows"
spec:
  capacity:
    storage: 30Gi
  accessModes:
    - ReadWriteOnce
  nfs:
    server: "nfs"
    path: /
  storageClassName: windows
EOF
  # Run only Windows tests
  export KUBEVIRT_E2E_FOCUS=Windows
elif [[ $TARGET =~ (cnao|multus) ]]; then
  export KUBEVIRT_E2E_FOCUS="Multus|Networking|VMIlifecycle|Expose|Macvtap"
elif [[ $TARGET =~ sig-network ]]; then
  export KUBEVIRT_E2E_FOCUS="\[sig-network\]"
elif [[ $TARGET =~ sriov.* ]]; then
  export KUBEVIRT_E2E_FOCUS=SRIOV
elif [[ $TARGET =~ gpu.* ]]; then
  export KUBEVIRT_E2E_FOCUS=GPU
elif [[ $TARGET =~ (okd|ocp).* ]]; then
  export KUBEVIRT_E2E_SKIP="SRIOV|GPU"
else
  export KUBEVIRT_E2E_SKIP="Multus|SRIOV|GPU|Macvtap"
fi

if [[ "$KUBEVIRT_STORAGE" == "rook-ceph" ]]; then
  export KUBEVIRT_E2E_FOCUS=rook-ceph
fi


# Run functional tests
FUNC_TEST_ARGS="$ginko_params -v -dryRun" make functest
