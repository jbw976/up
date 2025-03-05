FROM gcr.io/distroless/base@sha256:ee694eefd7685d8c443fec6abd3bff8e30c437faa8fbeacc0ce4c2e847d45501

ARG TARGETOS
ARG TARGETARCH

COPY bin/$TARGETOS\_$TARGETARCH/up /usr/local/bin/
COPY bin/$TARGETOS\_$TARGETARCH/docker-credential-up /usr/local/bin/
USER 65532
ENTRYPOINT ["up"]
