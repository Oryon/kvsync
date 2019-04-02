#!/bin/bash

cd $GOPATH
go test -v -gcflags "-N -l" github.com/Oryon/kvsync/encoding github.com/Oryon/kvsync/sync github.com/Oryon/kvsync/store
