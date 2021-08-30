#!/bin/bash -e

function finish {
  rc=$?
  if jobs 1 | grep -q Running; then
    echo
    echo "Killing proxy (pid=$proxy_pid)..."
    kill $proxy_pid
  fi
  exit $rc
}

# creates/removes prow/sriov resources for the given $node
function patch_node {
  if [ $pfs -ne 0 ]; then
    curl --header "Content-Type: application/json-patch+json" \
      --request PATCH \
      --data '[{"op": "add", "path": "/status/capacity/prow~1sriov", "value": "'$pfs'"}]' \
      http://localhost:8001/api/v1/nodes/$node/status
  else
    curl --header "Content-Type: application/json-patch+json" \
      --request PATCH \
      --data '[{"op": "remove", "path": "/status/capacity/prow~1sriov"}]' \
      http://localhost:8001/api/v1/nodes/$node/status
  fi
}

function main() {
  node=$1
  pfs=$2
  if [ -z $node ] || [ -z $pfs ]; then
    echo "syntax error, use: $0 <NODE_NAME> <PFS>"
    exit 1
  fi

  kubectl proxy &
  proxy_pid=$!
  sleep 3
  jobs 1 | grep -q Running

  trap finish EXIT SIGINT

  patch_node
}

main "$@"
