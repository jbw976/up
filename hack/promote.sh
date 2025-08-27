#!/bin/sh

# This script is used by our CI to promote releases. We currently maintain three
# release channels:
#
# - main: Every build from the main branch.
# - alpha: Release candidates for the next minor version. Stable releases should
#          also go here until an RC for the next minor is created.
# - stable: Releases for the current minor version.
#
# For now, we don't have a good way to maintain multiple stable minor versions
# at once. Every release that gets promoted to a channel becomes that channel's
# "current" release, which is what will be fetched by the install.sh script.

set -e

if [ -z "$CHANNEL" ]; then
    echo "CHANNEL must be non-empty"
    exit 1
fi
if [ -z "$S3_BUCKET" ]; then
    echo "S3_BUCKET must be non-empty"
    exit 1
fi
if [ -z "$OCI_REPOSITORY" ]; then
    echo "OCI_REPOSITORY must be non-empty"
    exit 1
fi

if [ -z "$VERSION" ]; then
    VERSION=$(git describe)
    echo "Defaulted VERSION to $VERSION"
fi

S3_FROM="s3://${S3_BUCKET}/build/${VERSION}"
S3_TO="s3://${S3_BUCKET}/${CHANNEL}/${VERSION}"
S3_CURRENT="s3://${S3_BUCKET}/${CHANNEL}/current"

set -x

# Promote artifacts in S3.
aws s3 sync --only-show-errors --delete "$S3_FROM" "$S3_TO"
aws s3 sync --only-show-errors --delete "$S3_TO" "$S3_CURRENT"

# Promote OCI images.
crane tag "$OCI_REPOSITORY:$VERSION" "$CHANNEL"
