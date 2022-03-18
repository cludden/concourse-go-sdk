FROM alpine:latest

RUN apk --update add ca-certificates

COPY check in out /opt/resource/

ENTRYPOINT ["/opt/resource/check"]
