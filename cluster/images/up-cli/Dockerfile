FROM gcr.io/distroless/base@sha256:125eb09bbd8e818da4f9eac0dfc373892ca75bec4630aa642d315ecf35c1afb7

ARG TARGETOS
ARG TARGETARCH

COPY bin/$TARGETOS\_$TARGETARCH/up /usr/local/bin/
COPY bin/$TARGETOS\_$TARGETARCH/docker-credential-up /usr/local/bin/
USER 65532
ENTRYPOINT ["up"]
