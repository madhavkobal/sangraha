# Dockerfile — wraps the pre-built linux/amd64 binary.
# The binary must be built before this image is built (see release.yml).
#
# Build:
#   make build
#   docker build -t sangraha .
#
# Run:
#   docker run -p 9000:9000 -p 9001:9001 \
#     -v /data/sangraha:/var/lib/sangraha \
#     sangraha

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.source="https://github.com/madhavkobal/sangraha"
LABEL org.opencontainers.image.description="S3-compatible single-binary object storage"
LABEL org.opencontainers.image.licenses="Apache-2.0"

COPY bin/sangraha /sangraha

# S3 API port
EXPOSE 9000
# Admin / web console port
EXPOSE 9001

VOLUME ["/var/lib/sangraha"]

ENTRYPOINT ["/sangraha"]
CMD ["server", "start"]
