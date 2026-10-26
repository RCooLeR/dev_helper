# PHP 8.0.30 from php-eol-patched (github.com/RCooLeR/php-eol-patched): the official image with
# selected security backports compiled in, plus what the local sites need on top.
# Settings come from conf/.
FROM ghcr.io/rcooler/php-eol-patched:8.0.30-debian@sha256:aadf968cc47f879019243dfce4d427bb3c79870f850cb4158d89d0aaed3e923d

COPY conf/opcache.ini conf/z-custom.ini /usr/local/etc/php/conf.d/

RUN apt-get update && apt-get install -y --no-install-recommends $PHPIZE_DEPS \
        git mc wget gnupg graphviz imagemagick ghostscript zbar-tools \
        libgmp-dev libsodium-dev unixodbc unixodbc-dev libgssapi-krb5-2 \
    && rm -rf /var/lib/apt/lists/*

# Bundled extensions the image leaves out, built from its patched source archive.
RUN [ -e /usr/include/gmp.h ] || ln -s /usr/include/$(gcc -print-multiarch)/gmp.h /usr/include/gmp.h; \
    docker-php-ext-install gmp ftp posix sodium \
    && docker-php-ext-configure pdo_odbc --with-pdo-odbc=unixODBC,/usr \
    && docker-php-ext-install pdo_odbc

# Microsoft ODBC driver 17 and sqlcmd for sqlsrv (Debian 11 packages, matching the image).
RUN curl -fsSL https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor -o /usr/share/keyrings/microsoft-prod.gpg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/microsoft-prod.gpg] https://packages.microsoft.com/debian/11/prod bullseye main" \
        > /etc/apt/sources.list.d/mssql-release.list \
    && apt-get update && ACCEPT_EULA=Y apt-get install -y msodbcsql17 mssql-tools \
    && rm -rf /var/lib/apt/lists/*
ENV PATH="${PATH}:/opt/mssql-tools/bin"

# PECL packages from their release URLs: this PEAR cannot parse pecl.php.net's chunked answers.
# 5.11.1 is the last sqlsrv for PHP 8.0.
RUN pecl install https://pecl.php.net/get/xdebug-3.5.3.tgz \
        https://pecl.php.net/get/sqlsrv-5.11.1.tgz https://pecl.php.net/get/pdo_sqlsrv-5.11.1.tgz \
    && docker-php-ext-enable xdebug sqlsrv pdo_sqlsrv

# Legacy remote services still need TLS 1.0 and security level 1.
RUN sed -i 's,^\(MinProtocol[ ]*=\).*,\1TLSv1.0,; s,^\(CipherString[ ]*=\).*,\1DEFAULT@SECLEVEL=1,' /etc/ssl/openssl.cnf

# WP-CLI is pinned before a release drops this PHP version.
RUN curl -fsSL https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer \
    && curl -fsSL -o /usr/local/bin/wp https://github.com/wp-cli/wp-cli/releases/download/v2.12.0/wp-cli-2.12.0.phar \
    && chmod +x /usr/local/bin/wp

# FPM workers run as the WSL user that owns the app files.
ARG PUID=1000
ARG PGID=1000
RUN groupmod -g ${PGID} www-data && usermod -u ${PUID} -g ${PGID} www-data

CMD ["php-fpm"]
