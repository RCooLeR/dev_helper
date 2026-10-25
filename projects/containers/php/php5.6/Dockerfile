# PHP 5.6.40 from php-eol-patched (github.com/RCooLeR/php-eol-patched): the official image with
# selected security backports compiled in, plus the usual tools on top. No site runs PHP 5.6;
# conf/ is a copy of the PHP 7.2 settings (PHP ignores newer directives).
FROM ghcr.io/rcooler/php-eol-patched:5.6.40-debian

COPY conf/php.ini /usr/local/etc/php/
COPY conf/www.conf /usr/local/etc/php-fpm.d/www.conf
# conf.d is read after php.ini, so the image's production defaults would override it.
RUN rm -f /usr/local/etc/php/conf.d/00-defaults.ini

RUN apt-get update && apt-get install -y --no-install-recommends $PHPIZE_DEPS \
        git wget mariadb-client libgmp-dev libmcrypt-dev \
    && rm -rf /var/lib/apt/lists/*

# Bundled extensions the image leaves out, built from its patched source archive.
RUN [ -e /usr/include/gmp.h ] || ln -s /usr/include/$(gcc -print-multiarch)/gmp.h /usr/include/gmp.h; \
    docker-php-ext-install gmp ftp posix mcrypt

# PECL packages from their release URLs: this PEAR cannot parse pecl.php.net's chunked answers.
# Xdebug 2.5 is the last line for PHP 5.6.
RUN pecl install https://pecl.php.net/get/xdebug-2.5.5.tgz \
    && docker-php-ext-enable xdebug

# Composer 2.2 is the last line that runs on PHP 5.6; WP-CLI is pinned before a release drops it.
RUN curl -fsSL https://getcomposer.org/installer | php -- --2.2 --install-dir=/usr/local/bin --filename=composer \
    && curl -fsSL -o /usr/local/bin/wp https://github.com/wp-cli/wp-cli/releases/download/v2.12.0/wp-cli-2.12.0.phar \
    && chmod +x /usr/local/bin/wp

# FPM workers run as the WSL user that owns the app files.
ARG PUID=1000
ARG PGID=1000
RUN groupmod -g ${PGID} www-data && usermod -u ${PUID} -g ${PGID} www-data

CMD ["php-fpm"]
