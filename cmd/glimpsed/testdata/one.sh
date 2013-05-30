
#!/bin/sh

cd `dirname $0`
i=1
j=1
k=1
l=1
./fixture.sh zone-$i product-$j env-$k name-$l $i "http http-mgmt http-info" |\
  protoc -I.. ../job.proto --encode glimpse.Job |\
  curl --data-binary @- -XPUT -HContent-Type:application/x-protobuf "http://localhost:8411/zone-$i/product-$j/env-$k/name-$l"
