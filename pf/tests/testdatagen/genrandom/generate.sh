#!/usr/bin/env bash

set -euo pipefail

mkdir -p $PWD/bin

HERE=$PWD

(cd $PWD/../../internal/cmd/pulumi-resource-random && go build -o $HERE/bin/pulumi-resource-random)

export PATH=$PWD/bin:$PATH
export PATH=~/.pulumi-dev/bin:$PATH

OUT=../../testdata/genrandom
mkdir -p $OUT

pulumi stack init generate
pulumi stack select generate

reindent()
{
    jq -s . "$1" > "$1.tmp"
    cp "$1.tmp" "$1"
    rm "$1.tmp"
}

retrace()
{
    prelog="$OUT/random-$2-preview.json"
    updlog="$OUT/random-$2-update.json"
    rm -rf "$prelog" "$updlog"

    pulumi config set min "$1"
    PULUMI_DEBUG_GRPC="$prelog" pulumi preview
    PULUMI_DEBUG_GRPC="$updlog" pulumi up --yes --skip-preview

    reindent "$prelog"
    reindent "$updlog"
}

retrace 1 initial
retrace 1 empty
retrace 2 replace
retrace 0 delete

pulumi destroy --yes
pulumi stack rm --yes
