FROM scratch
WORKDIR /var/run/sanderson-notifications
ENTRYPOINT ["/docker-run.sh"]
COPY docker-run.sh /
COPY sanderson-notifications /
