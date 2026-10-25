# PHP 7.1.33 from php-eol-patched (github.com/RCooLeR/php-eol-patched): the official image with
# selected security backports compiled in, plus what the local sites need on top.
# Settings come from conf/.
FROM ghcr.io/rcooler/php-eol-patched:7.1.33-debian

COPY conf/php.ini /usr/local/etc/php/
COPY conf/www.conf /usr/local/etc/php-fpm.d/www.conf
# conf.d is read after php.ini, so the image's production defaults would override it.
RUN rm -f /usr/local/etc/php/conf.d/00-defaults.ini

RUN apt-get update && apt-get install -y --no-install-recommends $PHPIZE_DEPS \
        git wget imagemagick graphicsmagick mariadb-client libgmp-dev libmcrypt-dev zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

# Bundled extensions the image leaves out, built from its patched source archive.
RUN [ -e /usr/include/gmp.h ] || ln -s /usr/include/$(gcc -print-multiarch)/gmp.h /usr/include/gmp.h; \
    docker-php-ext-install gmp ftp posix mcrypt

# PECL packages from their release URLs: this PEAR cannot parse pecl.php.net's chunked answers.
RUN pecl install https://pecl.php.net/get/xdebug-2.9.8.tgz https://pecl.php.net/get/memcache-4.0.5.2.tgz \
    && docker-php-ext-enable xdebug memcache

RUN cd /usr/src && curl -fsSL http://www.xmailserver.org/libxdiff-0.22.tar.gz | tar xz \
    && cd libxdiff-0.22 && ./configure && make && make install && ldconfig \
    && cd /usr/src && rm -rf libxdiff-0.22 \
    && pecl install https://pecl.php.net/get/xdiff-2.2.0.tgz \
    && docker-php-ext-enable xdiff

# Legacy remote services still need TLS 1.0 and security level 1.
RUN sed -i 's,^\(MinProtocol[ ]*=\).*,\1TLSv1.0,; s,^\(CipherString[ ]*=\).*,\1DEFAULT@SECLEVEL=1,' /etc/ssl/openssl.cnf

# Composer 2.2 is the last line that runs on PHP 7.1; WP-CLI is pinned before a release drops it.
RUN curl -fsSL https://getcomposer.org/installer | php -- --2.2 --install-dir=/usr/local/bin --filename=composer \
    && curl -fsSL -o /usr/local/bin/wp https://github.com/wp-cli/wp-cli/releases/download/v2.12.0/wp-cli-2.12.0.phar \
    && chmod +x /usr/local/bin/wp

# FPM workers run as the WSL user that owns the app files.
ARG PUID=1000
ARG PGID=1000
RUN groupmod -g ${PGID} www-data && usermod -u ${PUID} -g ${PGID} www-data

CMD ["php-fpm"]
