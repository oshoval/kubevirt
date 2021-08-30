#!/usr/bin/env bash

set -e

expected_component="network"
expected_managedby="bomba"
expected_partof="sweet"
expected_version="v1"

resources=( $(oc api-resources --no-headers -o=name | grep -wv bindings | grep -wv tokenreviews.authentication.k8s.io | grep -vw localsubjectaccessreviews.authorization.k8s.io) )

empty_array=("limitranges")
empty_array+=("persistentvolumeclaims")
empty_array+=("podtemplates")
empty_array+=("replicationcontrollers")
empty_array+=("resourcequotas")
empty_array+=("validatingwebhookconfigurations.admissionregistration.k8s.io")
empty_array+=("statefulsets.apps")
empty_array+=("horizontalpodautoscalers.autoscaling")
empty_array+=("cronjobs.batch")
empty_array+=("jobs.batch")
empty_array+=("bgpconfigurations.crd.projectcalico.org")
empty_array+=("bgppeers.crd.projectcalico.org")
empty_array+=("globalnetworkpolicies.crd.projectcalico.org")
empty_array+=("globalnetworksets.crd.projectcalico.org")
empty_array+=("hostendpoints.crd.projectcalico.org")
empty_array+=("ipamconfigs.crd.projectcalico.org")
empty_array+=("networkpolicies.crd.projectcalico.org")
empty_array+=("networksets.crd.projectcalico.org")
empty_array+=("ingresses.extensions")
empty_array+=("network-attachment-definitions.k8s.cni.cncf.io")
empty_array+=("ingressclasses.networking.k8s.io")
empty_array+=("ingresses.networking.k8s.io")
empty_array+=("networkpolicies.networking.k8s.io")
empty_array+=("nodenetworkconfigurationenactments.nmstate.io")
empty_array+=("nodenetworkconfigurationpolicies.nmstate.io")
empty_array+=("runtimeclasses.node.k8s.io")
empty_array+=("podsecuritypolicies.policy")
empty_array+=("csidrivers.storage.k8s.io")
empty_array+=("volumeattachments.storage.k8s.io")

containsElement () {
  local e match="$1"
  shift
  for e; do [[ "$e" == "$match" ]] && return 0; done
  return 1
}

for i in "${resources[@]}"
do
   if [ "$i" = "selfsubjectaccessreviews.authorization.k8s.io" ]; then 
      continue
   fi
   if [ "$i" = "selfsubjectrulesreviews.authorization.k8s.io" ]; then 
      continue
   fi
   if [ "$i" = "subjectaccessreviews.authorization.k8s.io" ]; then 
      continue
   fi

   if containsElement "$i" "${empty_array[@]}"; then
      continue
   fi
    
   cmd="oc get -n cluster-network-addons $i --no-headers -oname"

   # configmaps
   # deployments.apps
   #if [[ "$i" == "configmaps" ]]; then
      #echo "$i"
      result=( $($cmd) )
      #echo "$result"

      # TODO compare to specific expected tag
      for e in "${result[@]}"
      do
         cmd="oc get -n cluster-network-addons $e -oyaml"
         result=$($cmd)
         component=$(echo "$result" | docker run --rm -i -v `pwd`:/workdir mikefarah/yq:3.3.4 yq r - metadata.labels[app.kubernetes.io/component])
         managedby=$(echo "$result" | docker run --rm -i -v `pwd`:/workdir mikefarah/yq:3.3.4 yq r - metadata.labels[app.kubernetes.io/managed-by])
         partof=$(echo "$result" | docker run --rm -i -v `pwd`:/workdir mikefarah/yq:3.3.4 yq r - metadata.labels[app.kubernetes.io/part-of])
         version=$(echo "$result" | docker run --rm -i -v `pwd`:/workdir mikefarah/yq:3.3.4 yq r - metadata.labels[app.kubernetes.io/version])
         if [ -z "$component" ] || [ -z "$managedby" ] || [ -z "$partof" ] || [ -z "$version" ]; then
           echo "BAD  | $cmd"
         else
           #echo "GOOD | $cmd"
           metadata=$(echo "$result" | docker run --rm -i -v `pwd`:/workdir mikefarah/yq:3.3.4 yq r - spec.template.metadata.labels)
           if [ ! -z "$metadata" ]; then
             component=$(echo "$result" | docker run --rm -i -v `pwd`:/workdir mikefarah/yq:3.3.4 yq r - spec.template.metadata.labels[app.kubernetes.io/component])
             managedby=$(echo "$result" | docker run --rm -i -v `pwd`:/workdir mikefarah/yq:3.3.4 yq r - spec.template.metadata.labels[app.kubernetes.io/managed-by])
             partof=$(echo "$result" | docker run --rm -i -v `pwd`:/workdir mikefarah/yq:3.3.4 yq r - spec.template.metadata.labels[app.kubernetes.io/part-of])
             version=$(echo "$result" | docker run --rm -i -v `pwd`:/workdir mikefarah/yq:3.3.4 yq r - spec.template.metadata.labels[app.kubernetes.io/version])
             if [ "$component" != "$expected_component" ] || [ "$managedby" != "$expected_managedby" ] || [ "$partof" != "$expected_partof" ] || [ "$version" != "$expected_version" ]; then
               echo "BAD | $cmd"
             #else
             #  echo "GOOD | $cmd"
             fi
           #else 
           #  echo "GOOD | $cmd"
           fi
         fi
      done
   #fi

   #echo "$cmd"

   # find empty
   #if [ -z "$RESULT" ]; then
   #   echo "$i"
   #fi
done

