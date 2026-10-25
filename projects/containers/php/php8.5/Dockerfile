# PHP 8.5 FPM image for the devhelper stack: same toolset as php8.4 (gd, intl, soap, pdo_*,
# imagick, memcached, redis, xdebug, sqlsrv, wp-cli, composer, node via nvm, mssql tools).
# Debian is pinned to bookworm so every PHP image shares one base and package set.
FROM php:8.5-fpm-bookworm

RUN touch ~/.bashrc && chmod +x ~/.bashrc

COPY ./conf/opcache.ini /usr/local/etc/php/conf.d/
COPY ./conf/z-custom.ini /usr/local/etc/php/conf.d/
COPY ./conf/ssmtp.conf /etc/ssmtp/

# Build dependencies and the core extensions that every site relies on. OPcache is compiled
# into PHP since 8.5 (no module to install); conf/opcache.ini still configures it.
RUN apt-get update && apt-get install -y --no-install-recommends \
        git mc wget curl ca-certificates openssl unzip zip gnupg \
        pkg-config build-essential autoconf bison re2c \
        libmemcached-dev libmemcached11 \
        libfreetype6-dev libjpeg62-turbo-dev libpng-dev zlib1g-dev \
        libmcrypt-dev libxml2-dev libzip-dev libonig-dev libsqlite3-dev \
        libicu-dev libpq-dev libgmp-dev libsodium-dev libmagickwand-dev \
        graphviz imagemagick ghostscript zbar-tools ssmtp \
        unixodbc unixodbc-dev libgssapi-krb5-2 \
    && update-ca-certificates \
    && ln -s /usr/include/$(gcc -print-multiarch)/gmp.h /usr/include/gmp.h \
    && docker-php-ext-configure gd --with-freetype --with-jpeg \
    && docker-php-ext-configure zip \
    && docker-php-ext-configure intl \
    && docker-php-ext-configure pdo_odbc --with-pdo-odbc=unixODBC,/usr \
    && docker-php-ext-install -j$(nproc) \
        gd pdo pdo_mysql pdo_pgsql pdo_odbc mysqli mbstring intl soap \
        gmp zip exif bcmath sodium pcntl ftp sockets \
    && docker-php-ext-enable soap gmp zip exif sodium pcntl

# PECL extensions. PHP 8.5 is new: a PECL package that has no release for it yet must not
# break the image. Each one is attempted on its own and reported at the end of the build.
RUN pecl channel-update pecl.php.net \
    && (pecl install imagick && docker-php-ext-enable imagick || echo "NOTICE: imagick not available for this PHP yet") \
    && (pecl install memcached && docker-php-ext-enable memcached || echo "NOTICE: memcached not available for this PHP yet") \
    && (pecl install redis && docker-php-ext-enable redis || echo "NOTICE: redis not available for this PHP yet") \
    && (pecl install xdebug && docker-php-ext-enable xdebug || echo "NOTICE: xdebug not available for this PHP yet") \
    && (pecl install pdo_sqlsrv sqlsrv && docker-php-ext-enable sqlsrv pdo_sqlsrv || echo "NOTICE: sqlsrv/pdo_sqlsrv not available for this PHP yet")

# WP-CLI and Composer, as in the other PHP images.
RUN curl --cacert /etc/ssl/certs/ca-certificates.crt -O https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar \
    && chmod +x wp-cli.phar \
    && mv wp-cli.phar /usr/local/bin/wp \
    && echo "alias wp='wp'" >> /root/.bashrc \
    && wp --info

RUN curl --cacert /etc/ssl/certs/ca-certificates.crt -sS https://getcomposer.org/installer | php -- \
        --filename=composer \
        --install-dir=/usr/local/bin \
    && echo "alias composer='composer'" >> /root/.bashrc \
    && composer self-update

# Node through nvm for theme and asset builds inside the container.
RUN wget -qO- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.8/install.sh | bash \
    && echo 'export NVM_DIR="$HOME/.nvm"' >> "$HOME/.bashrc" \
    && echo '[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"  # This loads nvm' >> "$HOME/.bashrc" \
    && echo '[ -s "$NVM_DIR/bash_completion" ] && . "$NVM_DIR/bash_completion" # This loads nvm bash_completion' >> "$HOME/.bashrc" \
    && bash -c 'source $HOME/.nvm/nvm.sh && nvm install stable && nvm use stable'

# Microsoft ODBC driver and command-line tools for the SQL Server projects. Microsoft's
# prod.list already names its keyring (/usr/share/keyrings/microsoft-prod.gpg), so the key
# goes there unchanged. The whole step is optional: a repository outage must not break the image.
RUN install -d -m 0755 /usr/share/keyrings \
    && curl -fsSL https://packages.microsoft.com/keys/microsoft.asc \
        | gpg --dearmor -o /usr/share/keyrings/microsoft-prod.gpg \
    && chmod a+r /usr/share/keyrings/microsoft-prod.gpg \
    && curl -fsSL https://packages.microsoft.com/config/debian/12/prod.list \
        > /etc/apt/sources.list.d/mssql-release.list \
    && (apt-get update \
        && ACCEPT_EULA=Y apt-get install -y msodbcsql18 mssql-tools18 \
        && echo 'export PATH="$PATH:/opt/mssql-tools18/bin"' >> ~/.bashrc \
        || echo "NOTICE: Microsoft ODBC driver not installed") \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Summary of what made it into the image, visible in the build log.
RUN php -v && php -m

VOLUME /tmp

# Match the host user so files written by PHP belong to the developer.
ARG PUID=1000
ARG PGID=1000
RUN if getent group www-data >/dev/null; then \
      groupmod -g ${PGID} www-data; \
    else \
      groupadd -g ${PGID} www-data; \
    fi \
 && usermod -u ${PUID} -g ${PGID} www-data

# Keep FPM startup independent of network access and Composer updates.
CMD ["php-fpm"]
