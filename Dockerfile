FROM busybox:1.35.0
WORKDIR /var/run/sanderson-notifications
ENTRYPOINT ["/docker-run.sh"]
ENV CHECK_INTERVAL=30
COPY docker-run.sh /
COPY sanderson-notifications /
