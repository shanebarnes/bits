#!/bin/bash

go_package=go1.9.2.darwin-amd64.tar.gz
cwd=$(pwd)
curl -O -L https://redirector.gvt1.com/edgedl/go/${go_package}
tar -xzf ${go_package}
export GOROOT_BOOTSTRAP=${cwd}/go

git clone https://github.com/golang/go go1.9.1
cp go1.9.1.tcpfastpath.diff go1.9.1/
cd go1.9.1
git checkout tags/go1.9.1

git apply go1.9.1.tcpfastpath.diff
cd src
bash buildall.bash darwin-amd64

cd ../bin
./go version 
