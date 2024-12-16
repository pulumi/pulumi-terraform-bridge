#!/usr/bin/env bash

set -euo pipefail

V="$1"
INPUT="$2"
OUTPUT="$3"

orig=github.com/opentofu/opentofu/internal/tfplugin${V}
dest=github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin${V}

cat "$INPUT" |
    sed -r "s#$orig#$dest#g" |
    sed -r "s#package tfplugin${V};#package tfplugin${V}_pulumi;#g" >"$OUTPUT"
