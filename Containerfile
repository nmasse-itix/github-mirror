FROM docker.io/library/alpine:3.15
RUN apk --no-cache add ca-certificates \
 && update-ca-certificates
ARG BUILT_ARTIFACT
ADD "$BUILT_ARTIFACT" /
ENTRYPOINT [ "/github-mirror" ]
CMD []

