FROM gcr.io/distroless/base@sha256:74ddbf52d93fafbdd21b399271b0b4aac1babf8fa98cab59e5692e01169a1348

ARG TARGETOS
ARG TARGETARCH

COPY bin/$TARGETOS\_$TARGETARCH/up /usr/local/bin/
COPY bin/$TARGETOS\_$TARGETARCH/docker-credential-up /usr/local/bin/
USER 65532
ENTRYPOINT ["up"]
