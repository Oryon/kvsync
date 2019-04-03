#!/bin/bash -e

CD="$( cd "$( dirname $0 )" && pwd )"
cd $CD/../

# Uncomment to use gopath instead of gomod
# cd $GOPATH

echo "===== Unit testing ====="
go test -v -gcflags "-N -l" github.com/Oryon/kvsync/encoding github.com/Oryon/kvsync/sync github.com/Oryon/kvsync/store

echo "===== Examples ====="
echo "Running examples/sync1"
go run github.com/Oryon/kvsync/examples/sync1 > /dev/null
