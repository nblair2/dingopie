FROM ghcr.io/nblair2/dnp3-container:latest

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      tcpdump \
      iptables && \
    rm -rf /var/lib/apt/lists/*
