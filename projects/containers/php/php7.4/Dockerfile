# PHP 7.4.33 from php-eol-patched (github.com/RCooLeR/php-eol-patched): the official image with
# selected security backports compiled in, plus what the local sites need on top.
# Settings come from conf/.
FROM ghcr.io/rcooler/php-eol-patched:7.4.33-debian

COPY conf/php.ini /usr/local/etc/php/
COPY conf/www.conf /usr/local/etc/php-fpm.d/www.conf
COPY conf/z-custom.ini /usr/local/etc/php/conf.d/
# conf.d is read after php.ini, so the image's production defaults would override it.
RUN rm -f /usr/local/etc/php/conf.d/00-defaults.ini

RUN apt-get update && apt-get install -y --no-install-recommends $PHPIZE_DEPS \
        git wget imagemagick mariadb-client ssmtp libgmp-dev libsodium-dev \
    && rm -rf /var/lib/apt/lists/*
# After installing ssmtp, whose package would otherwise replace this file.
COPY conf/ssmtp.conf /etc/ssmtp/

# Bundled extensions the image leaves out, built from its patched source archive.
RUN [ -e /usr/include/gmp.h ] || ln -s /usr/include/$(gcc -print-multiarch)/gmp.h /usr/include/gmp.h; \
    docker-php-ext-install gmp ftp posix sodium

# PECL packages from their release URLs: this PEAR cannot parse pecl.php.net's chunked answers.
RUN pecl install https://pecl.php.net/get/xdebug-3.1.6.tgz \
    && docker-php-ext-enable xdebug

# WP-CLI is pinned before a release drops this PHP version.
RUN curl -fsSL https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer \
    && curl -fsSL -o /usr/local/bin/wp https://github.com/wp-cli/wp-cli/releases/download/v2.12.0/wp-cli-2.12.0.phar \
    && chmod +x /usr/local/bin/wp

RUN curl -fsSL https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.8/install.sh | bash \
    && bash -c '. "$HOME/.nvm/nvm.sh" && nvm install stable && nvm alias default stable'

# FPM workers run as the WSL user that owns the app files.
ARG PUID=1000
ARG PGID=1000
RUN groupmod -g ${PGID} www-data && usermod -u ${PUID} -g ${PGID} www-data

CMD ["php-fpm"]
