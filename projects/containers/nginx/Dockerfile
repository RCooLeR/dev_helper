FROM nginx:stable

# Keep OpenSSL available for local TLS tooling.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends openssl ca-certificates; \
    rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    mkdir -p /etc/nginx/external; \
    chmod 755 /etc/nginx/external

COPY ./cobf.d/00-default.conf /etc/nginx/conf.d/default.conf

COPY ./ssl/cert.pem /etc/nginx/external/cert.pem
COPY ./ssl/key.pem /etc/nginx/external/key.pem
COPY ./ssl/dh4096.pem /etc/nginx/external/dh.pem
COPY ./ssl/dh4096.pem /etc/nginx/external/dhparam.pem

RUN set -eux; \
    chmod 644 /etc/nginx/external/cert.pem /etc/nginx/external/dh.pem; \
    chmod 600 /etc/nginx/external/key.pem
