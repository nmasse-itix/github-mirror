FROM scratch
ARG BUILT_ARTIFACT
ADD "$BUILT_ARTIFACT" /
COPY ca-bundle.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT [ "/github-mirror" ]
CMD []

