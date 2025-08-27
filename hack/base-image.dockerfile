# This Dockerfile is used to build a base image for our `up` images, which are
# built by goreleaser using ko. The base image is identical to our provider-base
# image (used for official providers) except that it includes a `cp` command
# that can be used to copy the `up` binary into a volume when run as an ArgoCD
# init container.
#
# For now, we manually publish this image to xpkg.upbound.io/upbound/up-cli-base
# by running:
#
#   docker buildx build --platform linux/amd64 --platform linux/arm64 --push \
#     -t xpkg.upbound.io/upbound/up-cli-base:v1.0.0 \
#     -f hack/base-image.dockerfile \
#     .

FROM busybox AS busybox
FROM xpkg.upbound.io/upbound/provider-base@sha256:d23697e028f65fcc35886fe9e875069c071f637a79d65821830d6bc71c975391

COPY --from=busybox /bin/cp /bin/cp
