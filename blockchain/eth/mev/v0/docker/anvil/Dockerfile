FROM ghcr.io/foundry-rs/foundry:latest

WORKDIR /app

# temporarily elevate
USER root
COPY docker/anvil/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh


ENTRYPOINT ["/app/entrypoint.sh"]
