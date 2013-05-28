#!/bin/sh

cd `dirname $0`

for i in {0..5}; do
for j in {0..4}; do
for k in {0..3}; do
for l in {0..2}; do
  ./fixture.sh zone-$i product-$j env-$k name-$l $i "http http-mgmt http-info" |\
    protoc -I.. ../job.proto --encode glimpse.Job |\
    curl --data-binary @- -XPUT -HContent-Type:application/x-protobuf "http://localhost:8411/zone-$i/product-$j/env-$k/name-$l"
done
done
done
done
