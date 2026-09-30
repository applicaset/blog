FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETOS
ARG TARGETARCH
COPY bin/content_${TARGETOS}_${TARGETARCH} /service

ENTRYPOINT ["/service"]
