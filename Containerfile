FROM docker.io/library/alpine:3.15
RUN apk --no-cache add ca-certificates \
 && update-ca-certificates
ARG BUILT_ARTIFACT
ADD "$BUILT_ARTIFACT" /
COPY ca-bundle.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT [ "/github-mirror" ]
CMD []

