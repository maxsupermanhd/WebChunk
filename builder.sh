#!/bin/bash

TRG_PKG='main'
BUILD_TIME=$(date +"%Y%m%d.%H%M%S")
CommitHash=N/A
GitTag=N/A

GV=$(git tag || echo 'N/A')
if [[ $GV =~ [^[:space:]]+ ]];
then
    GitTag=${BASH_REMATCH[0]}
fi

GH=$(git log -1 --pretty=format:%h || echo 'N/A')
if [[ GH =~ 'fatal' ]];
then
    CommitHash=N/A
else
    CommitHash=$GH
fi

FLAG="-X $TRG_PKG.BuildTime=$BUILD_TIME"
FLAG="$FLAG -X $TRG_PKG.CommitHash=$CommitHash"
FLAG="$FLAG -X $TRG_PKG.GitTag=$GitTag"

echo 'go build'
go build -v -ldflags "$FLAG" -o webchunk
