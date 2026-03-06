FROM php:7.4-fpm-buster

# FIX: полностью переписываем sources.list для Debian Buster Archive
RUN printf "deb http://archive.debian.org/debian buster main contrib non-free\n\
deb http://archive.debian.org/debian buster-updates main contrib non-free\n\
deb http://archive.debian.org/debian-security buster/updates main contrib non-free\n" \
> /etc/apt/sources.list && \
echo 'Acquire::Check-Valid-Until "false";' > /etc/apt/apt.conf.d/99no-check-valid-until

COPY ./conf/php.ini /usr/local/etc/php/
COPY ./conf/www.conf /usr/local/etc/php-fpm.d/www.conf
COPY ./conf/z-custom.ini /usr/local/etc/php/conf.d/
COPY ./conf/ssmtp.conf /etc/ssmtp/

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
			libicu-dev

RUN  docker-php-ext-install pdo \
	&& docker-php-ext-install pdo_mysql \
	&& docker-php-ext-install mysqli \
	&& docker-php-ext-install opcache \
	&& docker-php-ext-install gd \
	&& docker-php-ext-install intl

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
    RUN docker-php-ext-configure zip
	RUN docker-php-ext-install zip && docker-php-ext-enable zip

	RUN apt-get update -y \
	  && apt-get install -y \
	    libxml2-dev \
	  && apt-get clean -y \
	  && docker-php-ext-install soap

RUN apt-get update && apt-get install -y libmagickwand-dev --no-install-recommends && rm -rf /var/lib/apt/lists/*
RUN pecl install imagick && docker-php-ext-enable imagick

RUN docker-php-ext-install bcmath
RUN docker-php-ext-enable xdebug

RUN apt-get update \
	&& apt-get install -y -q --no-install-recommends \
		msmtp msmtp-mta \
	&& apt-get clean \
	&& rm -r /var/lib/apt/lists/*

	RUN  curl -O https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar && \
			 chmod +x wp-cli.phar && \
			 mv wp-cli.phar /usr/local/bin/wp && \
			 echo "alias wp='wp'" >> /root/.bashrc && \
			 wp --info

RUN curl -sS https://getcomposer.org/installer | php -- \
        --filename=composer \
        --install-dir=/usr/local/bin && \
        echo "alias composer='composer'" >> /root/.bashrc

RUN wget -qO- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.5/install.sh | bash

RUN echo 'export NVM_DIR="$HOME/.nvm"'                                       >> "$HOME/.bashrc"
RUN echo '[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"  # This loads nvm' >> "$HOME/.bashrc"
RUN echo '[ -s "$NVM_DIR/bash_completion" ] && . "$NVM_DIR/bash_completion" # This loads nvm bash_completion' >> "$HOME/.bashrc"

RUN bash -c 'source $HOME/.nvm/nvm.sh   && \
    nvm install stable && nvm use stable';


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

CMD ["sh", "-c", "composer self-update && php-fpm"]