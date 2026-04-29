{
  idx = index($0, "\": ")
  if (idx > 0) {
    key_part = substr($0, 1, idx + 2)
    val_part = substr($0, idx + 3)
    gsub(/OpenShift/, "OpenStack", val_part)
    gsub(/openshift/, "openstack", val_part)
    gsub(/OPENSHIFT/, "OPENSTACK", val_part)
    printf "%s%s\n", key_part, val_part
  } else { print }
}
