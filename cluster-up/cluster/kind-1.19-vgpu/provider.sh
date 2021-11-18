#!/usr/bin/env bash

set -e

DEFAULT_CLUSTER_NAME="vgpu"
DEFAULT_HOST_PORT=5000
ALTERNATE_HOST_PORT=5001
export CLUSTER_NAME=${CLUSTER_NAME:-$DEFAULT_CLUSTER_NAME}

if [ $CLUSTER_NAME == $DEFAULT_CLUSTER_NAME ]; then
    export HOST_PORT=$DEFAULT_HOST_PORT
else
    export HOST_PORT=$ALTERNATE_HOST_PORT
fi

function set_kind_params() {
    export KIND_VERSION="${KIND_VERSION:-0.11.1}"
    export KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-quay.io/kubevirtci/kindest_node:v1.19.11@sha256:cbecc517bfad65e368cd7975d1e8a4f558d91160c051d0b1d10ff81488f5fb06}"
    export KUBECTL_PATH="${KUBECTL_PATH:-/bin/kubectl}"
}

function up() {
    # load the vfio_mdev module
    /usr/sbin/modprobe vfio_mdev
    
    # print hardware info for easier debugging based on logs
    echo 'Available cards'
    docker run --rm --cap-add=SYS_RAWIO quay.io/phoracek/lspci@sha256:0f3cacf7098202ef284308c64e3fc0ba441871a846022bb87d65ff130c79adb1 sh -c "lspci -k | grep -EA2 'VGA|3D'"
    echo ""

    cp $KIND_MANIFESTS_DIR/kind.yaml ${KUBEVIRTCI_CONFIG_PATH}/$KUBEVIRT_PROVIDER/kind.yaml
    _add_worker_kubeadm_config_patch
    _add_worker_extra_mounts
    kind_up

    # remove the rancher.io kind default storageClass
    _kubectl delete sc standard

    ${KUBEVIRTCI_PATH}/cluster/$KUBEVIRT_PROVIDER/config_vgpu_cluster.sh

    echo "$KUBEVIRT_PROVIDER cluster '$CLUSTER_NAME' is ready"
}

set_kind_params

source ${KUBEVIRTCI_PATH}/cluster/kind/common.sh

