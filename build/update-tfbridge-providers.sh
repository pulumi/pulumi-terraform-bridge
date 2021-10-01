#!/usr/bin/env bash
set -euo pipefail

RESOURCE_FILE="provider/resources.go"
MAKEFILE="Makefile"

SED=gsed

# If this fails, the script will exit
$SED --version > /dev/null

if [ ! -f $MAKEFILE ]; then
    echo "Failed to find $MAKEFEIL"
    exit 1
fi

if [ ! -f $RESOURCE_FILE ]; then
    echo "Failed to find $RESOURCE_FILE"
    exit 1
fi

# Edit the makefile
# Remove version insert
$SED -i 's/ -ldflags "-X ${PROJECT}\/${VERSION_PATH}=${VERSION}"//g' $MAKEFILE
# Add version export
VERSION_LINE_NUMBER=$(grep '^VERSION \+:=' $MAKEFILE -n | $SED 's/:.*//')
if [ -e "$VERSION_LINE_NUMBER" ""]; then
    echo "Failed to find version line in $MAKEFILE"
    exit 1
fi
$SED -i "$VERSION_LINE_NUMBER a export PULUMI_MAJOR_VERSION := VERSION" $MAKEFILE
# Insert a new line between the export and the "VERSION :=" line
$SED -i "$VERSION_LINE_NUMBER a \ " $MAKEFILE

echo "$MAKEFILE converted"

# Change current major version to majorVersion
$SED -i -e 's/tfbridge.GetModuleMajorVersion(version.Version)/majorVersion/g' $RESOURCE_FILE

# Respect PULUMI_MAJOR_VERSION in RESOURCE_FILE
PROVIDER_FUNC_LINE=$(
    grep 'func .*\(\) tfbridge.ProviderInfo {' $RESOURCE_FILE -n |
        $SED 's/:.*//')

if [ "$PROVIDER_FUNC_LINE" = "" ]; then
    echo "Failed to find provider function line in $RESOURCE_FILE"
    exit 1
fi

INSERT=$(cat <<-EOF
   var majorVersion string
    pulumiMajorVersion := os.Getenv("PULUMI_MAJOR_VERSION")
    if pulumiMajorVersion != "" {
        majorVersion = tfbridge.GetModuleMajorVersion(pulumiMajorVersion)
    } else {
        majorVersion = tfbridge.GetModuleMajorVersion(version.Version)
    }

EOF
)
INSERT=${INSERT//$'\n'/\\n}

# Insert the major version component into the beginning of the go file
$SED -i -e "$PROVIDER_FUNC_LINE a \ $INSERT\n" $RESOURCE_FILE

echo "$RESOURCE_FILE converted"
