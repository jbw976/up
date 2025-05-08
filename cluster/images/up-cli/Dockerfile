FROM busybox AS busybox
FROM gcr.io/distroless/base@sha256:fa5f94fa433728f8df3f63363ffc8dec4adcfb57e4d8c18b44bceccfea095ebc

ARG TARGETOS
ARG TARGETARCH

COPY --from=busybox /bin/cp /bin/cp
COPY bin/$TARGETOS\_$TARGETARCH/up /usr/local/bin/
COPY bin/$TARGETOS\_$TARGETARCH/docker-credential-up /usr/local/bin/
USER 65532
ENTRYPOINT ["up"]
