FROM php:8.0-fpm

COPY ./conf/opcache.ini /usr/local/etc/php/conf.d/
COPY ./conf/z-custom.ini /usr/local/etc/php/conf.d/

RUN apt-get update && apt-get install -y git mc \
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


RUN ln -s /usr/include/$(gcc -print-multiarch)/gmp.h /usr/include/gmp.h
RUN docker-php-ext-install gmp && docker-php-ext-enable gmp
RUN docker-php-ext-configure zip && docker-php-ext-install zip  && docker-php-ext-enable zip

RUN apt-get install -y pkg-config build-essential autoconf bison re2c libxml2-dev libsqlite3-dev

RUN curl -L -o /usr/local/bin/pickle https://github.com/FriendsOfPHP/pickle/releases/download/v0.7.9/pickle.phar \
	&& chmod +x /usr/local/bin/pickle

RUN apt-get -y install libmagickwand-dev ghostscript  --no-install-recommends \
	&&   pickle install imagick


RUN pickle install memcached && docker-php-ext-enable memcached
RUN apt-get -y install unixodbc-dev
RUN pecl channel-update pecl.php.net
RUN pecl install pdo_sqlsrv-5.11.1 sqlsrv-5.11.1
RUN printf "; priority=20\nextension=sqlsrv.so\n" > /usr/local/etc/php/conf.d/sqlsrv.ini
RUN printf "; priority=20\nextension=pdo_sqlsrv.so\n" > /usr/local/etc/php/conf.d/pdo_sqlsrv.ini

RUN apt-get install unixodbc unixodbc-dev -y \
 && docker-php-ext-configure pdo_odbc --with-pdo-odbc=unixODBC,/usr \
 && docker-php-ext-install pdo_odbc


RUN apt-get install -y gnupg
RUN curl -fsSL https://packages.microsoft.com/keys/microsoft.asc | apt-key add -
RUN curl https://packages.microsoft.com/config/debian/10/prod.list > /etc/apt/sources.list.d/mssql-release.list
RUN apt-get update
RUN ACCEPT_EULA=Y apt-get install -y msodbcsql17
RUN ACCEPT_EULA=Y apt-get install -y mssql-tools
RUN echo 'export PATH="$PATH:/opt/mssql-tools/bin"' >> ~/.bashrc
RUN apt-get install -y libgssapi-krb5-2


RUN apt-get update -y \
    && sed -i 's,^\(MinProtocol[ ]*=\).*,\1'TLSv1.0',g' /etc/ssl/openssl.cnf \
    && sed -i 's,^\(CipherString[ ]*=\).*,\1'DEFAULT@SECLEVEL=1',g' /etc/ssl/openssl.cnf

	RUN apt-get update -y \
	  && apt-get install -y \
	    libxml2-dev \
	  && apt-get clean -y \
	  && docker-php-ext-install soap && docker-php-ext-enable soap


RUN apt-get install -y zbar-tools

RUN printf "; priority=20\nextension=imagick.so\n" > /usr/local/etc/php/conf.d/imagick.ini

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

CMD ["sh", "-c", "composer self-update && php-fpm"]