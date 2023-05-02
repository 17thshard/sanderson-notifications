FROM alpine as certs
RUN apk update && apk add ca-certificates

FROM busybox:1.35.0
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

WORKDIR /var/run/sanderson-notifications
ENTRYPOINT ["/docker-run.sh"]
ENV CHECK_INTERVAL=30
COPY docker-run.sh /
COPY sanderson-notifications /
