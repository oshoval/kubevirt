#!/bin/bash

docker tag example-fedora:32 localhost:5000/kubevirt/fedora-sriov-lane-container-disk:devel
docker push localhost:5000/kubevirt/fedora-sriov-lane-container-disk:devel
