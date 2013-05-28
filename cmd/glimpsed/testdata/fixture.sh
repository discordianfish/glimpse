#!/bin/sh

zone=$1; shift
product=$1; shift
env=$1; shift
name=$1; shift

instances=$1; shift
services=$1; shift

echo zone: \"$zone\"
echo product: \"$product\"
echo env: \"$env\"
echo name: \"$name\"
while [ $instances != 0 ]; do
  instances=`expr $instances - 1`
  echo "instance: {"
  echo " index: $instances"
  for sv in $services; do
    echo " endpoint: {"
      echo "  " name: \"$sv\"
      echo "  " host: \"localhost\"
      echo "  " port: 7000
    echo " }"
  done
  echo "}"
done
