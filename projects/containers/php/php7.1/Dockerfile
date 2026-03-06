FROM php:7.1-fpm

# FIX: Debian Buster/Stretch archives
RUN sed -i 's/deb.debian.org/archive.debian.org/g' /etc/apt/sources.list && \
    sed -i 's|security.debian.org|archive.debian.org/|g' /etc/apt/sources.list && \
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
		graphicsmagick \
		libgraphicsmagick1-dev \
		libpq-dev \
      	mariadb-client \
	&& docker-php-ext-install pdo \
	&& docker-php-ext-install mbstring \
	&& docker-php-ext-install pdo_mysql \
	&& docker-php-ext-install mysqli \
	&& docker-php-ext-install opcache \
	&& docker-php-ext-install gd

	RUN pecl install redis-5.2.1 \
	    && pecl install xdebug-2.9.4 \
	    && docker-php-ext-enable redis xdebug

	RUN git clone -b php7 https://github.com/php-memcached-dev/php-memcached /usr/src/php/ext/memcached \
			    && docker-php-ext-configure /usr/src/php/ext/memcached \
			        --disable-memcached-sasl \
			    && docker-php-ext-install /usr/src/php/ext/memcached \
			    && rm -rf /usr/src/php/ext/memcached

	RUN git clone https://github.com/websupport-sk/pecl-memcache.git \
				&& cd pecl-memcache \
				&& git checkout -b php7 origin/php7 \
				&& phpize \
				&& ./configure \
				&& make \
				&& make install \
				&& docker-php-ext-enable memcache

	RUN apt-get update && apt-get install  -y libgmp-dev
	RUN ln -s /usr/include/$(gcc -print-multiarch)/gmp.h /usr/include/gmp.h
	RUN docker-php-ext-install gmp
	RUN docker-php-ext-configure zip && docker-php-ext-install zip  && docker-php-ext-enable zip

RUN apt-get update && apt-get install -y libmagickwand-dev --no-install-recommends && rm -rf /var/lib/apt/lists/*
RUN pecl install imagick && docker-php-ext-enable imagick


RUN docker-php-ext-install bcmath
RUN docker-php-ext-install mcrypt

RUN cd /usr/src \
    && wget http://www.xmailserver.org/libxdiff-0.22.tar.gz \
    && tar -xzf libxdiff-0.22.tar.gz \
    && cd libxdiff-0.22 \
    && ./configure \
    && make \
    && make install

RUN pecl install xdiff && docker-php-ext-enable xdiff

RUN apt-get update -y \
    && sed -i 's,^\(MinProtocol[ ]*=\).*,\1'TLSv1.0',g' /etc/ssl/openssl.cnf \
    && sed -i 's,^\(CipherString[ ]*=\).*,\1'DEFAULT@SECLEVEL=1',g' /etc/ssl/openssl.cnf

	RUN apt-get update -y \
	  && apt-get install -y \
	    libxml2-dev \
	  && apt-get clean -y \
	  && docker-php-ext-install soap && docker-php-ext-enable soap

RUN curl -sS https://getcomposer.org/installer | php -- \
        --filename=composer \
        --install-dir=/usr/local/bin && \
        echo "alias composer='composer'" >> /root/.bashrc

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
