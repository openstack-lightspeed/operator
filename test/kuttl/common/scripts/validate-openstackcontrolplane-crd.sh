#!/bin/bash

for i in {1..5}; do
    echo "Attempt $i: Waiting for CRD to be established..."
    if oc wait --for=condition=Established crd/openstackcontrolplanes.core.openstack.org --timeout=20s; then
        echo "CRD is Established."
        exit 0
    fi
    echo "Status field not ready yet, retrying in 3s..."
    sleep 3
done

exit 1
