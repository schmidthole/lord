FROM registry:latest

COPY config.yml /etc/docker/registry/config.yml
COPY htpasswd /etc/docker/registry/htpasswd

EXPOSE 80

CMD ["registry", "serve", "/etc/docker/registry/config.yml"]
