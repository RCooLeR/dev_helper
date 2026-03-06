FROM php:7.3-fpm-buster

# Full replace: Debian Buster archive repos
RUN set -eux; \
    printf '%s\\n' \
      'deb http://archive.debian.org/debian buster main contrib non-free' \
      'deb http://archive.debian.org/debian buster-updates main contrib non-free' \
      'deb http://archive.debian.org/debian-security buster/updates main contrib non-free' \
      > /etc/apt/sources.list; \
    echo 'Acquire::Check-Valid-Until "false";' > /etc/apt/apt.conf.d/99no-check-valid-until
	
COPY ./conf/php.ini /usr/local/etc/php/
COPY ./conf/www.conf /usr/local/etc/php-fpm.d/www.conf

RUN apt-get update && apt-get install -y git \
	libmemcached-dev \
	libmemcached11 \
	wget \
    	libmemcached-dev \
    	libfreetype6-dev \
    	libjpeg62-turbo-dev \
    	libmcrypt-dev \
    	libpng-dev \
    	imagemagick \
	&& docker-php-ext-install pdo \
	&& docker-php-ext-install mbstring \
	&& docker-php-ext-install pdo_mysql \
	&& docker-php-ext-install mysqli \
	&& docker-php-ext-install opcache \
	&& docker-php-ext-configure gd --with-png-dir=/usr --with-jpeg-dir=/usr \
	&& docker-php-ext-install gd

RUN pecl install redis-4.3.0 \
	    && pecl install xdebug-2.9.8 \
	    && docker-php-ext-enable redis

			RUN pecl install memcached \
&& docker-php-ext-enable memcached

RUN apt-get update && apt-get install  -y libgmp-dev
RUN ln -s /usr/include/$(gcc -print-multiarch)/gmp.h /usr/include/gmp.h
RUN docker-php-ext-install gmp

RUN apt-get install -y libzip-dev \
	zip
RUN docker-php-ext-configure zip --with-libzip
RUN docker-php-ext-install zip && docker-php-ext-enable zip

RUN apt-get update -y \
&& apt-get install -y \
libxml2-dev \
&& apt-get clean -y \
&& docker-php-ext-install soap

RUN apt-get update && apt-get install -y libmagickwand-dev --no-install-recommends && rm -rf /var/lib/apt/lists/*
RUN pecl install imagick && docker-php-ext-enable imagick

RUN apt-get update && apt-get install -y zbar-tools
RUN docker-php-ext-install bcmath
RUN docker-php-ext-enable xdebug

RUN  curl -O https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar && \
		 chmod +x wp-cli.phar && \
		 mv wp-cli.phar /usr/local/bin/wp && \
		 echo "alias wp='wp'" >> /root/.bashrc && \
		 wp --info

RUN curl -sS https://getcomposer.org/installer | php -- \
        --filename=composer \
        --install-dir=/usr/local/bin && \
        echo "alias composer='composer'" >> /root/.bashrc

VOLUME /tmp

# Fix fucking permissions
ARG PUID=1000
ARG PGID=1000

# Ensure group exists or adjust it
RUN if getent group www-data >/dev/null; then \
      groupmod -g ${PGID} www-data; \
    else \
      groupadd -g ${PGID} www-data; \
    fi \
 && usermod -u ${PUID} -g ${PGID} www-data

CMD ["composer", "self-update", "--allow-root"]
CMD ["php-fpm"]
