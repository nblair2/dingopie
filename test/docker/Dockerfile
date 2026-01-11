FROM ghcr.io/nblair2/opendnp3/demo

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      tcpdump \
      iptables && \
    rm -rf /var/lib/apt/lists/*
