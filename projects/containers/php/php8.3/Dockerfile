FROM php:8.3-fpm

RUN touch ~/.bashrc && chmod +x ~/.bashrc

COPY ./conf/opcache.ini /usr/local/etc/php/conf.d/
COPY ./conf/z-custom.ini /usr/local/etc/php/conf.d/
COPY ./conf/ssmtp.conf /etc/ssmtp/

RUN apt-get update && apt-get install -y git mc\
	libmemcached-dev \
	libmemcached11 \
	wget \
    	libmemcached-dev \
			libfreetype6-dev \
			libjpeg62-turbo-dev \
			 libmcrypt-dev \
			 libpng-dev \
			 zlib1g-dev \
			 libxml2-dev \
			 libzip-dev \
			 zip \
			 libonig-dev \
			 graphviz \
    		 imagemagick \
			 libpq-dev \
			 libgmp-dev \
			 pkg-config build-essential autoconf bison re2c libxml2-dev libsqlite3-dev \
			 libicu-dev \
	&& docker-php-ext-install opcache \
	&& docker-php-ext-configure gd --with-freetype --with-jpeg \
	&& docker-php-ext-install -j$(nproc) gd \
	&& docker-php-ext-install pdo \
	&& docker-php-ext-install mbstring \
	&& docker-php-ext-install pdo_mysql \
	&& docker-php-ext-install pdo_pgsql \
	&& docker-php-ext-install mysqli \
	&& docker-php-ext-install opcache \
	&& docker-php-ext-install intl \
	&& docker-php-ext-install soap && docker-php-ext-enable soap


RUN ln -s /usr/include/x86_64-linux-gnu/gmp.h /usr/include/gmp.h
RUN docker-php-ext-install gmp && docker-php-ext-enable gmp
RUN docker-php-ext-configure zip && docker-php-ext-install zip  && docker-php-ext-enable zip

RUN apt-get install -y pkg-config build-essential autoconf bison re2c libxml2-dev libsqlite3-dev

RUN curl -L -o /usr/local/bin/pickle https://github.com/FriendsOfPHP/pickle/releases/download/v0.7.9/pickle.phar \
	&& chmod +x /usr/local/bin/pickle

RUN apt-get -y install libmagickwand-dev ghostscript  --no-install-recommends \
	&&   pickle install imagick
	
RUN pickle install memcached && docker-php-ext-enable memcached

	RUN apt-get update -y \
	  && apt-get install -y \
	    libxml2-dev \
	  && apt-get clean -y \
	  && docker-php-ext-install soap && docker-php-ext-enable soap

RUN docker-php-ext-install exif && docker-php-ext-enable exif
RUN pecl install imagick; \
    docker-php-ext-enable imagick;

	RUN apt-get install -y zbar-tools


	RUN apt-get update \
		&& apt-get install -y -q --no-install-recommends \
			ssmtp \
		&& apt-get clean \
		&& rm -r /var/lib/apt/lists/*

RUN docker-php-ext-install bcmath
RUN pecl install xdebug
RUN docker-php-ext-enable xdebug

RUN  curl -O https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar && \
		 chmod +x wp-cli.phar && \
		 mv wp-cli.phar /usr/local/bin/wp && \
		 echo "alias wp='wp'" >> /root/.bashrc && \
		 wp --info

RUN curl -sS https://getcomposer.org/installer | php -- \
        --filename=composer \
        --install-dir=/usr/local/bin && \
        echo "alias composer='composer'" >> /root/.bashrc && \
        composer self-update

RUN wget -qO- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash

RUN echo 'export NVM_DIR="$HOME/.nvm"'                                       >> "$HOME/.bashrc"
RUN echo '[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"  # This loads nvm' >> "$HOME/.bashrc"
RUN echo '[ -s "$NVM_DIR/bash_completion" ] && . "$NVM_DIR/bash_completion" # This loads nvm bash_completion' >> "$HOME/.bashrc"

RUN bash -c 'source $HOME/.nvm/nvm.sh   && \
    nvm install stable && nvm use stable';

RUN pecl install redis
RUN docker-php-ext-enable redis

RUN apt-get -y update && apt-get install -y --no-install-recommends libsodium-dev

RUN docker-php-ext-install  sodium pcntl 
RUN docker-php-ext-enable  sodium pcntl 


RUN apt-get -y update && apt-get install -y  --allow-unauthenticated --fix-missing libicu-dev && docker-php-ext-configure intl && docker-php-ext-install intl

RUN docker-php-ext-install ftp

RUN docker-php-ext-install sockets 

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

CMD ["php-fpm"]
