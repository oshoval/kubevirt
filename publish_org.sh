#!/bin/bash

docker tag quay.io/kubevirtci/fedora-sriov-testing localhost:5000/kubevirt/fedora-sriov-lane-container-disk:devel
docker push localhost:5000/kubevirt/fedora-sriov-lane-container-disk:devel
