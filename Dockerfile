FROM gcr.io/distroless/static-debian11
WORKDIR /var/run/sanderson-notifications
ENTRYPOINT ["/docker-run.sh"]
ENV CHECK_INTERVAL=30
COPY docker-run.sh /
COPY sanderson-notifications /
