#!/bin/bash

while true; do
    csv=$(oc get subscription openstack-lightspeed-operator -n openshift-lightspeed -o jsonpath='{.status.installedCSV}' 2>/dev/null)
    if [ -n "$csv" ]; then
        echo "Found installedCSV: $csv"
        break
    fi
    echo "Waiting for openstack-lightspeed-operator Subscription installedCSV to be populated ..."
    sleep 5
done

# Wait for the CSV to succeed
oc wait csv $csv --for=jsonpath='{.status.phase}'=Succeeded --timeout=300s -n openshift-lightspeed
