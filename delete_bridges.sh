#!/bin/bash -e

for i in {1..100}
do
cat <<EOL | kubectl create -f -
apiVersion: nmstate.io/v1alpha1
kind: NodeNetworkConfigurationPolicy
metadata:
  name: br${i}
spec:
  desiredState:
    interfaces:
    - name: br${i}
      description: Linux bridge
      type: linux-bridge
      state: absent
      bridge:
        port:
        - name: eth1
EOL
done
